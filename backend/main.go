package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"ai-info/poc/moss/backend/database"
	"ai-info/poc/moss/backend/engine"

	"github.com/gin-gonic/gin"
)

type ChatRequest struct {
	UserName string `json:"user_name" binding:"required"`
	Text     string `json:"text" binding:"required"`
}

type ChatResponse struct {
	FinalResponse   string                `json:"final_response"`
	FactContext     string                `json:"fact_context"`
	TraceSteps      []AgentStep           `json:"trace_steps"`
	ConfirmRequired bool                  `json:"confirm_required"`
	ConflictingID   int64                 `json:"conflicting_id"`
	PendingFact     *engine.ExtractedFact `json:"pending_fact"`
	ConflictType    string                `json:"conflict_type"`
}

type ConfirmRequest struct {
	UserName      string               `json:"user_name" binding:"required"`
	ConflictingID int64                `json:"conflicting_id"`
	PendingFact   engine.ExtractedFact `json:"pending_fact" binding:"required"`
	ConflictType  string               `json:"conflict_type"`
}

type AgentStep struct {
	AgentName string `json:"agent_name"`
	Input     string `json:"input"`
	Output    string `json:"output"`
}

func main() {
	log.Println("MOSS GoLang 백엔드 서버 기동 준비...")

	// 1. 데이터베이스 초기화 (SQLite)
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./moss_memory.db"
	}
	db, err := database.InitDB(dbPath)
	if err != nil {
		log.Fatalf("데이터베이스 초기화 실패: %v", err)
	}
	defer db.Close()
	log.Printf("SQLite 데이터베이스 연결 완료: %s", dbPath)

	// 2. Gin 라우터 구성
	r := gin.Default()

	// CORS 허용 설정
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	// 3. API 엔드포인트 등록
	r.POST("/api/chat", func(c *gin.Context) {
		var req ChatRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "유효하지 않은 요청 데이터 포맷입니다."})
			return
		}

		// MOSS 에이전트 워크플로우 실행
		result, factCtx, traceSteps, linterResult, err := engine.RunMossWorkflow(c.Request.Context(), db, req.UserName, req.Text)
		if err != nil {
			log.Printf("[API ERROR] RunMossWorkflow 실패 (사용자: %s): %v", req.UserName, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// engine.AgentStep 슬라이스를 main.AgentStep 슬라이스로 복사 및 가공
		apiSteps := make([]AgentStep, len(traceSteps))
		for i, s := range traceSteps {
			apiSteps[i] = AgentStep{
				AgentName: s.AgentName,
				Input:     s.Input,
				Output:    s.Output,
			}
		}

		confirmRequired := false
		var conflictingID int64
		var pendingFact *engine.ExtractedFact
		var conflictType string

		if linterResult != nil && linterResult.Status == "CONFIRM_REQUIRED" {
			confirmRequired = true
			conflictingID = linterResult.ConflictingID
			pendingFact = linterResult.PendingFact
			conflictType = linterResult.ConflictType
		}

		c.JSON(http.StatusOK, ChatResponse{
			FinalResponse:   result,
			FactContext:     factCtx,
			TraceSteps:      apiSteps,
			ConfirmRequired: confirmRequired,
			ConflictingID:   conflictingID,
			PendingFact:     pendingFact,
			ConflictType:    conflictType,
		})
	})

	// 일정 덮어쓰기 최종 승인 API
	r.POST("/api/memories/confirm", func(c *gin.Context) {
		var req ConfirmRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "유효하지 않은 요청 포맷입니다."})
			return
		}

		// 1. 기존 충돌 일정 만료 처리 (연차와 겹치는 경우에는 연차를 만료시키지 않고 회의만 추가)
		if req.ConflictingID > 0 && req.ConflictType != "VACATION_OVERLAP" {
			log.Printf("[API Confirm] 기존 충돌 일정 만료 처리 개시 - ID: %d", req.ConflictingID)
			err := database.InvalidateMemories(db, req.UserName, []int64{req.ConflictingID})
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("기존 일정 만료 처리 실패: %v", err)})
				return
			}
		} else if req.ConflictType == "VACATION_OVERLAP" {
			log.Printf("[API Confirm] 연차와 중복되어 기존 연차(ID: %d)를 만료시키지 않고 회의만 추가합니다.", req.ConflictingID)
		}

		// 2. 대기 중이던 신규 팩트 DB 영구 저장
		rec := &database.MemoryRecord{
			UserName:    req.UserName,
			Domain:      req.PendingFact.Domain,
			Subject:     req.PendingFact.Subject,
			Predicate:   req.PendingFact.Predicate,
			ObjectValue: req.PendingFact.ObjectValue,
			StartTime:   req.PendingFact.StartTime,
			EndTime:     req.PendingFact.EndTime,
		}

		err = database.SaveMemory(db, rec)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("신규 일정 저장 실패: %v", err)})
			return
		}

		log.Printf("[API Confirm Success] 사용자 컨펌 완료로 신규 일정 저장됨 - ID: %d, 값: %s", rec.ID, rec.ObjectValue)
		c.JSON(http.StatusOK, gin.H{
			"message": "성공적으로 일정이 업데이트 되었습니다.",
			"record":  rec,
		})
	})

	// DB 모니터링용 전체 기억 조회 API
	r.GET("/api/memories", func(c *gin.Context) {
		userName := c.DefaultQuery("user_name", "yundream")
		
		records, err := database.FindAllMemories(db, userName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if records == nil {
			records = []database.MemoryRecord{}
		}
		c.JSON(http.StatusOK, records)
	})

	// 특정 ID 기억 영구 삭제 API
	r.DELETE("/api/memories/:id", func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "잘못된 ID 형식입니다."})
			return
		}

		if err := database.DeleteMemoryByID(db, id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "성공적으로 삭제되었습니다."})
	})

	// 4. 서버 포트 8080 기동
	port := ":8080"
	log.Printf("MOSS 백엔드 API 서버 포트 %s 에서 서빙을 시작합니다.", port)
	if err := r.Run(port); err != nil {
		log.Fatalf("서버 구동 실패: %v", err)
	}
}
