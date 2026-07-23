package storage

import (
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

type DB struct {
	WriteDB *sql.DB
	ReadDB  *sql.DB
}

// NewDB khởi tạo SQLite connection pool tối ưu (Single Writer + Reader Pool)
func NewDB(dbPath string) (*DB, error) {
	// Write Connection: MaxOpenConns = 1 để tránh SQLite write lock contention
	writeDB, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open write db: %w", err)
	}
	writeDB.SetMaxOpenConns(1)
	writeDB.SetMaxIdleConns(1)

	// Read Connection Pool: Cho phép concurrent read nhiều connections
	readDB, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_synchronous=NORMAL&_query_only=true&_busy_timeout=5000")
	if err != nil {
		writeDB.Close()
		return nil, fmt.Errorf("failed to open read db: %w", err)
	}
	readDB.SetMaxOpenConns(8)
	readDB.SetMaxIdleConns(8)

	// Cấu hình PRAGMAs tối ưu hiệu năng
	writePragmas := []string{
		"PRAGMA journal_mode = WAL;",
		"PRAGMA synchronous = NORMAL;",
		"PRAGMA temp_store = MEMORY;",
		"PRAGMA busy_timeout = 5000;",
		"PRAGMA foreign_keys = ON;",
		"PRAGMA cache_size = -32000;", // 32MB cache
	}

	for _, pragma := range writePragmas {
		if _, err := writeDB.Exec(pragma); err != nil {
			log.Printf("Warning setting write pragma '%s': %v", pragma, err)
		}
	}

	// PRAGMAs cho ReadDB (bỏ qua journal_mode và foreign_keys vì là query_only)
	readPragmas := []string{
		"PRAGMA temp_store = MEMORY;",
		"PRAGMA busy_timeout = 5000;",
		"PRAGMA cache_size = -32000;", // 32MB cache
	}

	for _, pragma := range readPragmas {
		if _, err := readDB.Exec(pragma); err != nil {
			log.Printf("Warning setting read pragma '%s': %v", pragma, err)
		}
	}

	db := &DB{
		WriteDB: writeDB,
		ReadDB:  readDB,
	}

	if err := db.InitSchema(); err != nil {
		return nil, fmt.Errorf("failed to init schema: %w", err)
	}

	return db, nil
}

func (db *DB) Close() {
	if db.WriteDB != nil {
		db.WriteDB.Close()
	}
	if db.ReadDB != nil {
		db.ReadDB.Close()
	}
}
