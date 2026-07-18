package database

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// InitDB는 SQLite 데이터베이스를 초기화하고 연결 인스턴스를 반환합니다.
func InitDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("SQLite 데이터베이스 열기 실패: %w", err)
	}

	// 커넥션 풀 헬스 체크
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("데이터베이스 핑 실패: %w", err)
	}

	// memory 테이블 스키마 초기화
	query := `
	CREATE TABLE IF NOT EXISTS memory (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_name TEXT NOT NULL,
		domain TEXT NOT NULL,
		subject TEXT NOT NULL,
		predicate TEXT NOT NULL,
		object_value TEXT NOT NULL,
		status TEXT NOT NULL CHECK(status IN ('active', 'inactive')),
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		start_time TEXT,
		end_time TEXT
	);
	`
	if _, err := db.Exec(query); err != nil {
		db.Close()
		return nil, fmt.Errorf("기억 테이블 초기화 실패: %w", err)
	}

	return db, nil
}
