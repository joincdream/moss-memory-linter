package engine

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"ai-info/poc/moss/backend/database"
)

// LinterResult는 린팅 검증 완료 후의 기계적 판정 결과를 담는 구조체입니다.
type LinterResult struct {
	Status           string // "SUCCESS", "BLOCKED", "SUCCESS_WITH_OVERWRITE", "CONFIRM_REQUIRED"
	ConflictType     string // "NONE", "VACATION_OVERLAP", "MEETING_OVERLAP"
	ConflictingID    int64
	ConflictingStart string
	ConflictingEnd   string
	TargetObject     string
	PendingFact      *ExtractedFact
}

// RunMemoryLinter는 신규 입력된 팩트들의 시간대 중복 및 충돌을 검사하고,
// 연차 기간과 회의 예약 간의 비즈니스 정합성을 강제합니다.
func RunMemoryLinter(db *sql.DB, userName string, facts []ExtractedFact) (LinterResult, error) {
	log.Printf("[MOSS Linter] 린팅 검사 개시 (사용자: %s, 신규 팩트 개수: %d)", userName, len(facts))

	result := LinterResult{
		Status:       "SUCCESS",
		ConflictType: "NONE",
	}

	for _, fact := range facts {
		if (fact.EndTime == nil || *fact.EndTime == "") && (fact.StartTime != nil && *fact.StartTime != "") {
			parsedStart, err := time.Parse("2006-01-02 15:04:05", *fact.StartTime)
			if err == nil {
				defaultEnd := parsedStart.Add(1 * time.Hour).Format("2006-01-02 15:04:05")
				fact.EndTime = &defaultEnd
				log.Printf("[MOSS Linter] 종료 시간 누락으로 기본 1시간 설정 적용: %s", defaultEnd)
			}
		}

		if fact.StartTime == nil || fact.EndTime == nil || *fact.StartTime == "" || *fact.EndTime == "" {
			continue
		}

		log.Printf("[MOSS Linter] 시간 충돌 검사 수행 범위: %s ~ %s", *fact.StartTime, *fact.EndTime)
		overlaps, err := database.FindOverlappingMemories(db, userName, *fact.StartTime, *fact.EndTime)
		if err != nil {
			return result, fmt.Errorf("시간대 중복 검증 조회 실패: %w", err)
		}

		// 1. 신규 등록하려는 팩트가 'meeting'인 경우
		if fact.Predicate == "meeting" {
			// (1) 연차 일정과 겹치는지 검사 -> 확인 필요 (CONFIRM_REQUIRED)
			hasVacationConflict := false
			var conflictingVacation database.MemoryRecord
			for _, over := range overlaps {
				if over.Predicate == "vacation" && over.Status == "active" {
					conflictingVacation = over
					hasVacationConflict = true
					break
				}
			}
			if hasVacationConflict {
				log.Printf("[MOSS Linter Alert] 연차 일정 감지 - ID: %d 로 인해 신규 회의 '%s' 등록에 확인이 필요합니다.", conflictingVacation.ID, fact.ObjectValue)
				var startStr, endStr string
				if conflictingVacation.StartTime != nil {
					startStr = *conflictingVacation.StartTime
				}
				if conflictingVacation.EndTime != nil {
					endStr = *conflictingVacation.EndTime
				}
				pending := fact
				return LinterResult{
					Status:           "CONFIRM_REQUIRED",
					ConflictType:     "VACATION_OVERLAP",
					ConflictingID:    conflictingVacation.ID,
					ConflictingStart: startStr,
					ConflictingEnd:   endStr,
					TargetObject:     conflictingVacation.ObjectValue,
					PendingFact:      &pending,
				}, nil
			}

			// (2) 다른 회의 일정과 겹치는지 검사 -> 승인 필요 (CONFIRM_REQUIRED)
			hasMeetingConflict := false
			var conflictingMeeting database.MemoryRecord
			for _, over := range overlaps {
				if over.Predicate == "meeting" && over.Status == "active" {
					conflictingMeeting = over
					hasMeetingConflict = true
					break
				}
			}
			if hasMeetingConflict {
				log.Printf("[MOSS Linter Alert] 회의 일정 중복 감지 - ID: %d 와 신규 회의 '%s' 충돌. 승인 보류.", conflictingMeeting.ID, fact.ObjectValue)
				var startStr, endStr string
				if conflictingMeeting.StartTime != nil {
					startStr = *conflictingMeeting.StartTime
				}
				if conflictingMeeting.EndTime != nil {
					endStr = *conflictingMeeting.EndTime
				}
				pending := fact
				return LinterResult{
					Status:           "CONFIRM_REQUIRED",
					ConflictType:     "MEETING_OVERLAP",
					ConflictingID:    conflictingMeeting.ID,
					ConflictingStart: startStr,
					ConflictingEnd:   endStr,
					TargetObject:     conflictingMeeting.ObjectValue,
					PendingFact:      &pending,
				}, nil
			}
		}

		// 2. 신규 등록하려는 팩트가 'vacation'인 경우 (회의 기간 내 신규 연차 등록 -> 기존 회의 자동 만료)
		if fact.Predicate == "vacation" {
			var overlappingIDs []int64
			var lastConflictingMeeting database.MemoryRecord
			for _, over := range overlaps {
				if over.Status == "active" {
					log.Printf("[MOSS Linter Warning] 충돌 데이터 감지 - ID: %d | Predicate: %s | 값: %s를 만료 처리합니다.", over.ID, over.Predicate, over.ObjectValue)
					overlappingIDs = append(overlappingIDs, over.ID)
					lastConflictingMeeting = over
				}
			}

			// 충돌 난 기존 일정 만료 처리
			if len(overlappingIDs) > 0 {
				err := database.InvalidateMemories(db, userName, overlappingIDs)
				if err != nil {
					return result, fmt.Errorf("기존 충돌 일정 만료 처리 실패: %w", err)
				}
				var startStr, endStr string
				if lastConflictingMeeting.StartTime != nil {
					startStr = *lastConflictingMeeting.StartTime
				}
				if lastConflictingMeeting.EndTime != nil {
					endStr = *lastConflictingMeeting.EndTime
				}
				result = LinterResult{
					Status:           "SUCCESS_WITH_OVERWRITE",
					ConflictType:     "MEETING_OVERWRITE",
					ConflictingID:    lastConflictingMeeting.ID,
					ConflictingStart: startStr,
					ConflictingEnd:   endStr,
					TargetObject:     lastConflictingMeeting.ObjectValue,
				}
			}
		}

		// 신규 팩트 저장
		rec := &database.MemoryRecord{
			UserName:    userName,
			Domain:      fact.Domain,
			Subject:     fact.Subject,
			Predicate:   fact.Predicate,
			ObjectValue: fact.ObjectValue,
			StartTime:   fact.StartTime,
			EndTime:     fact.EndTime,
		}

		err = database.SaveMemory(db, rec)
		if err != nil {
			return result, fmt.Errorf("신규 기억 저장 실패 (Subject: %s): %w", fact.Subject, err)
		}
		log.Printf("[MOSS Linter Success] 신규 기억 저장 완료 - ID: %d | Subject: %s | 값: %s",
			rec.ID, rec.Subject, rec.ObjectValue)
	}

	return result, nil
}
