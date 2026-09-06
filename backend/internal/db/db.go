package db

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaFS embed.FS

type DB struct {
	SQL *sql.DB
}

func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", filepath.ToSlash(path))
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetConnMaxLifetime(time.Hour)

	schema, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("read schema: %w", err)
	}
	if _, err := sqlDB.Exec(string(schema)); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}

	store := &DB{SQL: sqlDB}
	if err := store.seedIfEmpty(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("seed: %w", err)
	}
	return store, nil
}

func (d *DB) Close() error {
	return d.SQL.Close()
}

func (d *DB) seedIfEmpty() error {
	var n int
	if err := d.SQL.QueryRow(`SELECT COUNT(*) FROM community_areas`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	csvPath, err := FindBaselinesCSV()
	if err != nil {
		return err
	}
	return SeedFromCSV(d.SQL, csvPath)
}
