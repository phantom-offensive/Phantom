package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Database wraps the SQLite connection.
type Database struct {
	conn *sql.DB
}

// Open initializes the SQLite database at the given path and runs migrations.
func Open(path string) (*Database, error) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// SQLite only allows one writer at a time; serialize access to avoid
	// SQLITE_BUSY under concurrent check-in/audit/task writes.
	conn.SetMaxOpenConns(1)

	// Enable foreign keys, WAL journaling and a busy timeout so concurrent
	// access waits briefly instead of failing immediately.
	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA journal_mode = WAL",
	}
	for _, p := range pragmas {
		if _, err := conn.Exec(p); err != nil {
			conn.Close()
			return nil, fmt.Errorf("apply %s: %w", p, err)
		}
	}

	db := &Database{conn: conn}

	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return db, nil
}

// Close closes the database connection.
func (db *Database) Close() error {
	return db.conn.Close()
}

// Conn returns the underlying sql.DB for direct queries.
func (db *Database) Conn() *sql.DB {
	return db.conn
}
