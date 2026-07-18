package engine

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"ai-info/poc/moss/backend/database"
)

// SynthesizeActiveContext는 SQLite 데이터베이스를 조회하여,
// 감지된 도메인에 부합하는 활성 기억 목록을 하나의 문자열로 결합해 반환합니다.
func SynthesizeActiveContext(db *sql.DB, userName string, domain string) (string, error) {
	log.Printf("[MOSS Synthesizer] 활성 기억 조회 개시 (사용자: %s, 도메인 필터: %s)", userName, domain)

	var records []database.MemoryRecord
	var err error

	if domain == "UNKNOWN" || domain == "" {
		records, err = database.FindActiveMemories(db, userName)
	} else {
		records, err = database.FindActiveMemoriesByDomain(db, userName, domain)
	}

	if err != nil {
		return "", fmt.Errorf("데이터베이스 활성 기억 조회 실패: %w", err)
	}

	// 유효 기억이 한 건도 없는 경우, 환각 방지용 가드 메시지 반환
	if len(records) == 0 {
		log.Println("[MOSS Synthesizer] 활성 기억이 존재하지 않습니다. 환각 가드 텍스트를 리턴합니다.")
		return "현재 유효하게 등록된 스마트홈 기기 작동 예약 및 일정 정보가 존재하지 않습니다.", nil
	}

	var sb strings.Builder
	sb.WriteString("아래는 데이터베이스 검증을 통과하여 현재 유효하게 활성화된 사실 정보(Fact Context) 목록이다:\n\n")

	for i, rec := range records {
		timeInfo := ""
		hasStart := rec.StartTime != nil && *rec.StartTime != ""
		hasEnd := rec.EndTime != nil && *rec.EndTime != ""

		if hasStart && hasEnd {
			timeInfo = fmt.Sprintf(" (예약 작동 기간: %s ~ %s)", *rec.StartTime, *rec.EndTime)
		} else if hasStart {
			timeInfo = fmt.Sprintf(" (예약 작동 시점: %s)", *rec.StartTime)
		} else if hasEnd {
			timeInfo = fmt.Sprintf(" (예약 만료 시점: %s)", *rec.EndTime)
		}

		sb.WriteString(fmt.Sprintf("%d. [%s] %s의 %s 속성은 '%s' 상태로 설정됨%s\n",
			i+1, rec.Domain, rec.Subject, rec.Predicate, rec.ObjectValue, timeInfo))
	}

	synthesizedText := sb.String()
	log.Printf("[MOSS Synthesizer Success] %d건의 활성 기억을 기반으로 컨텍스트 합성 완료", len(records))
	return synthesizedText, nil
}
