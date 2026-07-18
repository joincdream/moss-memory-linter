package engine

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/workflow"
)

// Go ADK 2.0 워크플로우 전역 인스턴스 뼈대 정의
var (
	analyserAgent workflow.Node
	mainAgent     workflow.Node
	mossWorkflow  *workflow.Workflow
)

func init() {
	log.Println("[MOSS DAG] Go ADK 2.0 워크플로우 v2 뼈대 빌드 시작...")

	// 1. 공통 빈 설정 정보 선언
	cfg := workflow.NodeConfig{}

	// 2. 질문 분석 에이전트
	analyserAgent = workflow.NewFunctionNode("analyser_agent", func(ctx agent.Context, input any) (any, error) {
		log.Println("[MOSS Analyser] Ollama 의도 분석 개시...")
		msg, ok := input.(string)
		if !ok {
			return nil, fmt.Errorf("분석 노드 입력이 문자열이 아닙니다: %T", input)
		}

		result, err := AnalyzeUserMessage(ctx, msg)
		if err != nil {
			log.Printf("[MOSS Analyser Error] 분석 실패: %v", err)
			return nil, err
		}
		return result, nil
	}, cfg)

	// 3. 메인 에이전트
	mainAgent = workflow.NewFunctionNode("main_agent", func(ctx agent.Context, input any) (any, error) {
		log.Println("[MOSS MainAgent] 메인 에이전트 가동...")
		return input, nil
	}, cfg)

	// 4. Go ADK 2.0 Graph Workflow 직선 엣지 체인 구축
	edges := workflow.Chain(workflow.Start, analyserAgent, mainAgent)

	// 5. workflow.New 생성
	var err error
	mossWorkflow, err = workflow.New("moss_poc_workflow", edges)
	if err != nil {
		log.Fatalf("[MOSS DAG] 워크플로우 인스턴스 빌드 실패: %v", err)
	}

	log.Println("[MOSS DAG] Go ADK 2.0 워크플로우 v2 뼈대 선언 및 엣지 구축 성공")
}

// 메인 에이전트 오염 방지 시스템 지침 프롬프트
const MainAgentInstruction = `너는 외부 Fact Context(데이터베이스에서 조회된 최종 활성 사실 및 시스템 린터 검증 결과)를 기반으로 사용자에게 처리 결과를 안내하는 전문 에이전트이다.

1. 등록/변경 요청 (WRITE)
   - [System Linter Result]의 Status가 "BLOCKED"인 경우:
     일정 등록에 실패(차단)했음을 명확히 안내하라.
     이유를 설명할 때는 ConflictType, ConflictingTimeRange 정보를 활용하여 사용자에게 겹치는 기존 일정 정보를 구체적으로 제시하라.
   - [System Linter Result]의 Status가 "CONFIRM_REQUIRED"인 경우:
     동일한 시간대에 겹치는 기존 일정이 있음을 사용자에게 정중히 안내하고 승인 여부를 물어보라.
     * ConflictType이 "VACATION_OVERLAP"인 경우:
       해당 시간에 연차(Vacation) 일정이 있음을 안내하고, 휴가 중이지만 급무로 인해 회의 일정을 추가로 중복 등록할 것인지 물어보라.
       (예: "내일은 연차 휴가 기간입니다. 휴가 중이지만 회의 예약을 강제 등록하여 추가하시겠습니까?")
     * ConflictType이 "MEETING_OVERLAP"인 경우:
       동일한 시간대에 다른 회의가 예약되어 있음을 알리고, 기존 회의를 취소하고 새 회의로 일정을 변경(덮어쓰기)할 것인지 물어보라.
       (예: "오전 10시에는 이미 '개발팀 미팅'이 예약되어 있습니다. 기존 일정을 취소하고 '미소제약 고객 미팅'으로 변경하시겠습니까?")
   - [System Linter Result]의 Status가 "SUCCESS" 또는 "SUCCESS_WITH_OVERWRITE"인 경우:
     성공적으로 일정이 등록 완료되었음을 알리고, 만약 Overwrite되어 기존 미팅 일정이 만료 처리되었다면 어떤 일정이 만료되었는지를 명확히 대조하여 안내하라.

2. 정보 조회 (QUERY)
   - 오직 주입된 Fact Context의 활성 데이터에만 근거하여 일관성 있게 답변하고, 없는 정보는 존재하지 않는다고 답변하라.`

// CallMainAgent는 최종 사실 컨텍스트와 사용자의 원본 발화를 모델에 전달해 안전한 요약 답변을 반환합니다.
func CallMainAgent(ctx context.Context, factContext string, userMsg string) (string, error) {
	// 현재 시각 정보를 명확한 타임스탬프로 주입
	currentTimePrompt := fmt.Sprintf("현재 시각: %s", time.Now().Format("2006-01-02 15:04:05 Monday"))

	// 프롬프트 조립
	fullPrompt := fmt.Sprintf("System:\n%s\n\nFact Context:\n%s\n\nUser Message:\n%s\n%s",
		MainAgentInstruction, factContext, userMsg, currentTimePrompt)

	return CallGeminiAPI(ctx, fullPrompt, false)
}

// RunMossWorkflow는 의도 분기 및 린팅, 컨텍스트 합성을 관장하는 오케스트레이터 역할을 수행합니다.
type AgentStep struct {
	AgentName string
	Input     string
	Output    string
}

func RunMossWorkflow(ctx context.Context, db *sql.DB, userName, text string) (string, string, []AgentStep, *LinterResult, error) {
	fmt.Printf("[MOSS DAG Run] user: %s | text: %s\n", userName, text)
	log.Printf("[MOSS DAG Run] %s 그래프 실행 진입", mossWorkflow.Name())

	var traceSteps []AgentStep

	// 1단계. 질문 분석기 기동
	result, err := AnalyzeUserMessage(ctx, text)
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("의도 분석 실패: %w", err)
	}

	resultJson, _ := json.MarshalIndent(result, "", "  ")
	traceSteps = append(traceSteps, AgentStep{
		AgentName: "analyser_agent",
		Input:     text,
		Output:    string(resultJson),
	})

	log.Printf("[MOSS DAG Run] 분석 의도 식별됨: %s | 추출 사실 개수: %d", result.Intent, len(result.ExtractedFacts))

	// 추출된 팩트들로부터 타겟 도메인 식별
	detectedDomain := ""
	if len(result.ExtractedFacts) > 0 {
		detectedDomain = result.ExtractedFacts[0].Domain
	}

	// 2단계. 의도에 따른 정석 제어 흐름 분기 (Dynamic Orchestration)
	switch result.Intent {
	case "WRITE":
		linterResult, err := RunMemoryLinter(db, userName, result.ExtractedFacts)
		if err != nil {
			return "", "", nil, nil, fmt.Errorf("린터 프로세스 에러: %w", err)
		}

		factContext, err := SynthesizeActiveContext(db, userName, detectedDomain)
		if err != nil {
			return "", "", nil, nil, fmt.Errorf("컨텍스트 조립 실패: %w", err)
		}

		// Linter 결과를 정규화된 템플릿 형태로 합산 주입
		var conflictingRange string
		if linterResult.ConflictingStart != "" || linterResult.ConflictingEnd != "" {
			conflictingRange = fmt.Sprintf("%s ~ %s", linterResult.ConflictingStart, linterResult.ConflictingEnd)
		} else {
			conflictingRange = "N/A"
		}

		linterText := fmt.Sprintf("[System Linter Result]\nStatus: %s\nConflictType: %s\nConflictingID: %d\nConflictingTimeRange: %s\nTargetObjectValue: %s\n\n",
			linterResult.Status, linterResult.ConflictType, linterResult.ConflictingID, conflictingRange, linterResult.TargetObject)

		factContext = linterText + factContext

		mainAgentInput := fmt.Sprintf("Fact Context:\n%s\n\nUser Message:\n%s", factContext, text)
		ans, err := CallMainAgent(ctx, factContext, text)
		if err == nil {
			traceSteps = append(traceSteps, AgentStep{
				AgentName: "main_agent",
				Input:     mainAgentInput,
				Output:    ans,
			})
		}
		return ans, factContext, traceSteps, &linterResult, err

	case "QUERY":
		factContext, err := SynthesizeActiveContext(db, userName, detectedDomain)
		if err != nil {
			return "", "", nil, nil, fmt.Errorf("컨텍스트 조립 실패: %w", err)
		}

		// QUERY는 차단이 없으므로 기본 SUCCESS 주입
		linterText := "[System Linter Result]\nStatus: SUCCESS\nConflictType: NONE\nConflictingID: 0\nConflictingTimeRange: N/A\nTargetObjectValue: N/A\n\n"
		factContext = linterText + factContext

		mainAgentInput := fmt.Sprintf("Fact Context:\n%s\n\nUser Message:\n%s", factContext, text)
		ans, err := CallMainAgent(ctx, factContext, text)
		if err == nil {
			traceSteps = append(traceSteps, AgentStep{
				AgentName: "main_agent",
				Input:     mainAgentInput,
				Output:    ans,
			})
		}
		return ans, factContext, traceSteps, nil, err

	case "UNKNOWN":
		log.Println("[MOSS DAG Run] 일반/정체성 질문 식별 - 조기 반환 처리 완료")
		traceSteps = append(traceSteps, AgentStep{
			AgentName: "main_agent (Short-circuit)",
			Input:     "No DB context required for UNKNOWN intent",
			Output:    result.Response,
		})
		return result.Response, "일반 질문 의도 식별 (DB 조회 스킵됨)", traceSteps, nil, nil

	default:
		return result.Response, "알 수 없는 의도 식별", traceSteps, nil, nil
	}
}
