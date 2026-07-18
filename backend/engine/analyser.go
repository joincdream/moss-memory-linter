package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ExtractedFact는 스마트홈 기기 제어 및 일정 예약에서 추출된 개별 매개변수 구조체입니다.
type ExtractedFact struct {
	Domain      string  `json:"domain"`
	Subject     string  `json:"subject"`
	Predicate   string  `json:"predicate"`
	ObjectValue string  `json:"object_value"`
	StartTime   *string `json:"start_time"`
	EndTime     *string `json:"end_time"`
}

// AnalysisResult는 질문 분석기가 최종적으로 반환하는 구조화된 JSON 데이터 스키마입니다.
type AnalysisResult struct {
	Intent         string          `json:"intent"`
	ExtractedFacts []ExtractedFact `json:"extracted_facts"`
	Response       string          `json:"response"`
}

// 질문 분석기 시스템 지침 프롬프트 (사내 일정 및 연차/회의 관리 텍사노미 적용)
const AnalyserInstruction = `사용자 입력에서 일정 관련 정보를 식별하여 JSON으로만 응답하라.

[시간 처리 규칙]
* 하단에 전달되는 "현재 시각"을 기준으로 오늘, 내일, 모레, 요일 등 모든 상대적 표현을 계산하여 SQL datetime 형식("YYYY-MM-DD HH:MM:SS")으로 출력하라.

[시맨틱 레이어 규칙 (Taxonomy)]
* intent: "WRITE" (등록/변경), "QUERY" (조회), "UNKNOWN" (기타 질문)
* domain: "Schedule" 고정
* subject: "Calendar" 고정
* predicate: "vacation" (휴가 시, value는 "Vacation"), "meeting" (회의 시, value는 회의 이름)

[출력 JSON 구조]
{
  "intent": "WRITE" | "QUERY" | "UNKNOWN",
  "extracted_facts": [
    {
      "domain": "Schedule",
      "subject": "Calendar",
      "predicate": "vacation" | "meeting",
      "object_value": string,
      "start_time": string | null,
      "end_time": string | null
    }
  ],
  "response": string | null
}
`

// AnalyzeUserMessage는 로컬 Ollama 모델에 프롬프트를 전송하여 의도를 파싱하고 정형화된 구조체를 반환합니다.
func AnalyzeUserMessage(ctx context.Context, userMsg string) (*AnalysisResult, error) {
	// 현재 시각 정보를 명확한 타임스탬프로 주입
	currentTimePrompt := fmt.Sprintf("현재 시각: %s", time.Now().Format("2006-01-02 15:04:05 Monday"))
	fullPrompt := fmt.Sprintf("System:\n%s\n\nUser Message:\n%s\n%s", AnalyserInstruction, userMsg, currentTimePrompt)

	response, err := CallGeminiAPI(ctx, fullPrompt, true)
	if err != nil {
		return nil, err
	}

	// 언어 모델이 뱉어낸 JSON을 최종 구조화 분석 결과로 언마샬링
	var result AnalysisResult
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("의도 분석 JSON 언마샬링 실패: %w | 원본 응답: %s", err, response)
	}

	return &result, nil
}
