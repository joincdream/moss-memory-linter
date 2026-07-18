package database

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// MemoryRecord는 데이터베이스의 memory 테이블 행을 나타내는 구조체입니다.
type MemoryRecord struct {
	ID          int64     `json:"id"`
	UserName    string    `json:"user_name"`
	Domain      string    `json:"domain"`
	Subject     string    `json:"subject"`
	Predicate   string    `json:"predicate"`
	ObjectValue string    `json:"object_value"`
	Status      string    `json:"status"`
	UpdatedAt   time.Time `json:"updated_at"`
	StartTime   *string   `json:"start_time"`
	EndTime     *string   `json:"end_time"`
}

// SaveMemory는 신규 기억 사실을 데이터베이스에 active 상태로 삽입합니다.
func SaveMemory(db *sql.DB, rec *MemoryRecord) error {
	query := `
	INSERT INTO memory (user_name, domain, subject, predicate, object_value, status, start_time, end_time)
	VALUES (?, ?, ?, ?, ?, 'active', ?, ?)
	`
	res, err := db.Exec(query, rec.UserName, rec.Domain, rec.Subject, rec.Predicate, rec.ObjectValue, rec.StartTime, rec.EndTime)
	if err != nil {
		return fmt.Errorf("기억 삽입 실패: %w", err)
	}

	id, err := res.LastInsertId()
	if err == nil {
		rec.ID = id
	}
	return nil
}

// InvalidateMemories는 지정한 ID들의 기억 상태를 inactive(만료)로 일괄 업데이트합니다.
func InvalidateMemories(db *sql.DB, userName string, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}

	// SQL 주입 공격을 차단하면서 동적 플레이스홀더를 조립합니다.
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids)+1)
	args[0] = userName

	for i, id := range ids {
		placeholders[i] = "?"
		args[i+1] = id
	}

	query := fmt.Sprintf(
		"UPDATE memory SET status = 'inactive' WHERE user_name = ? AND id IN (%s)",
		strings.Join(placeholders, ","),
	)

	_, err := db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("기억 상태 만료 업데이트 실패: %w", err)
	}
	return nil
}

// FindActiveMemories는 특정 사용자의 활성 상태인 모든 기억 목록을 조회합니다.
func FindActiveMemories(db *sql.DB, userName string) ([]MemoryRecord, error) {
	// 기준 시간(2026-07-12)에 의거하여 이번 주(일요일~토요일) 기본 조회 범위 연산
	baseTime := time.Date(2026, 7, 12, 0, 0, 0, 0, time.Local)
	weekday := int(baseTime.Weekday())
	startOfWeek := baseTime.AddDate(0, 0, -weekday)             // 이번 주 일요일 00:00:00
	endOfWeek := startOfWeek.AddDate(0, 0, 7).Add(-time.Second) // 이번 주 토요일 23:59:59

	startStr := startOfWeek.Format("2006-01-02 15:04:05")
	endStr := endOfWeek.Format("2006-01-02 15:04:05")

	query := `
	SELECT id, user_name, domain, subject, predicate, object_value, status, updated_at, start_time, end_time
	FROM memory
	WHERE user_name = ? AND status = 'active'
	  AND (domain != 'Schedule' OR (domain = 'Schedule' AND start_time >= ? AND start_time <= ?))
	ORDER BY updated_at DESC
	`
	rows, err := db.Query(query, userName, startStr, endStr)
	if err != nil {
		return nil, fmt.Errorf("활성 기억 조회 실패: %w", err)
	}
	defer rows.Close()

	var records []MemoryRecord
	for rows.Next() {
		var rec MemoryRecord
		var updatedAtStr string
		err := rows.Scan(
			&rec.ID, &rec.UserName, &rec.Domain, &rec.Subject,
			&rec.Predicate, &rec.ObjectValue, &rec.Status,
			&updatedAtStr, &rec.StartTime, &rec.EndTime,
		)
		if err != nil {
			return nil, fmt.Errorf("행 스캔 실패: %w", err)
		}

		// Datetime 파싱 (SQLite의 문자열 형식을 Go의 time.Time으로 변환)
		if t, err := time.Parse("2006-01-02 15:04:05", updatedAtStr); err == nil {
			rec.UpdatedAt = t
		} else if t, err := time.Parse(time.RFC3339, updatedAtStr); err == nil {
			rec.UpdatedAt = t
		}
		records = append(records, rec)
	}
	return records, nil
}

// FindActiveMemoriesByDomain은 특정 사용자의 특정 도메인(Domain)에 속한 활성 기억 목록을 조회합니다.
func FindActiveMemoriesByDomain(db *sql.DB, userName string, domain string) ([]MemoryRecord, error) {
	// 기준 시간(2026-07-12)에 의거하여 이번 주(일요일~토요일) 기본 조회 범위 연산
	baseTime := time.Date(2026, 7, 12, 0, 0, 0, 0, time.Local)
	weekday := int(baseTime.Weekday())
	startOfWeek := baseTime.AddDate(0, 0, -weekday)             // 이번 주 일요일 00:00:00
	endOfWeek := startOfWeek.AddDate(0, 0, 7).Add(-time.Second) // 이번 주 토요일 23:59:59

	startStr := startOfWeek.Format("2006-01-02 15:04:05")
	endStr := endOfWeek.Format("2006-01-02 15:04:05")

	// 도메인 필터 추가
	query := `
	SELECT id, user_name, domain, subject, predicate, object_value, status, updated_at, start_time, end_time
	FROM memory
	WHERE user_name = ? AND status = 'active' AND domain = ?
	  AND (domain != 'Schedule' OR (domain = 'Schedule' AND start_time >= ? AND start_time <= ?))
	ORDER BY updated_at DESC
	`
	rows, err := db.Query(query, userName, domain, startStr, endStr)
	if err != nil {
		return nil, fmt.Errorf("도메인별 활성 기억 조회 실패: %w", err)
	}
	defer rows.Close()

	var records []MemoryRecord
	for rows.Next() {
		var rec MemoryRecord
		var updatedAtStr string
		err := rows.Scan(
			&rec.ID, &rec.UserName, &rec.Domain, &rec.Subject,
			&rec.Predicate, &rec.ObjectValue, &rec.Status,
			&updatedAtStr, &rec.StartTime, &rec.EndTime,
		)
		if err != nil {
			return nil, fmt.Errorf("행 스캔 실패: %w", err)
		}

		if t, err := time.Parse("2006-01-02 15:04:05", updatedAtStr); err == nil {
			rec.UpdatedAt = t
		} else if t, err := time.Parse(time.RFC3339, updatedAtStr); err == nil {
			rec.UpdatedAt = t
		}
		records = append(records, rec)
	}
	return records, nil
}


// FindOverlappingMemories는 새로 등록하려는 일정 범위와 겹치는 기존 활성 기억들을 수학적으로 조회합니다.
// 겹침 공식: S1 < E2 AND S2 < E1 (기존의 시작시간이 새 종료시간보다 작고, 기존의 종료시간이 새 시작시간보다 큰 경우)
func FindOverlappingMemories(db *sql.DB, userName string, startTime, endTime string) ([]MemoryRecord, error) {
	query := `
	SELECT id, user_name, domain, subject, predicate, object_value, status, updated_at, start_time, end_time
	FROM memory
	WHERE user_name = ? AND status = 'active' AND start_time < ? AND end_time > ?
	`
	rows, err := db.Query(query, userName, endTime, startTime)
	if err != nil {
		return nil, fmt.Errorf("시간대 중복 기억 조회 실패: %w", err)
	}
	defer rows.Close()

	var records []MemoryRecord
	for rows.Next() {
		var rec MemoryRecord
		var updatedAtStr string
		err := rows.Scan(
			&rec.ID, &rec.UserName, &rec.Domain, &rec.Subject,
			&rec.Predicate, &rec.ObjectValue, &rec.Status,
			&updatedAtStr, &rec.StartTime, &rec.EndTime,
		)
		if err != nil {
			return nil, fmt.Errorf("행 스캔 실패: %w", err)
		}
		records = append(records, rec)
	}
	return records, nil
}

// FindAllMemories는 특정 사용자의 활성 및 만료된 모든 기억 목록을 조회합니다. (모니터링용)
func FindAllMemories(db *sql.DB, userName string) ([]MemoryRecord, error) {
	query := `
	SELECT id, user_name, domain, subject, predicate, object_value, status, updated_at, start_time, end_time
	FROM memory
	WHERE user_name = ?
	ORDER BY id DESC
	`
	rows, err := db.Query(query, userName)
	if err != nil {
		return nil, fmt.Errorf("전체 기억 조회 실패: %w", err)
	}
	defer rows.Close()

	var records []MemoryRecord
	for rows.Next() {
		var rec MemoryRecord
		var updatedAtStr string
		err := rows.Scan(
			&rec.ID, &rec.UserName, &rec.Domain, &rec.Subject,
			&rec.Predicate, &rec.ObjectValue, &rec.Status,
			&updatedAtStr, &rec.StartTime, &rec.EndTime,
		)
		if err != nil {
			return nil, fmt.Errorf("행 스캔 실패: %w", err)
		}
		if t, err := time.Parse("2006-01-02 15:04:05", updatedAtStr); err == nil {
			rec.UpdatedAt = t
		}
		records = append(records, rec)
	}
	return records, nil
}

// DeleteMemoryByID는 특정 ID에 속하는 기억 레코드를 영구히 삭제합니다.
func DeleteMemoryByID(db *sql.DB, id int64) error {
	query := "DELETE FROM memory WHERE id = ?"
	_, err := db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("기억 삭제 실패: %w", err)
	}
	return nil
}

