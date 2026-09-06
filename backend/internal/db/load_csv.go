package db

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
)

// SeedFromCSV loads community areas and hourly baselines from the public-mobility
// style CSV shipped with the repo (Chicago TNP hour-of-week aggregates).
func SeedFromCSV(sqlDB *sql.DB, csvPath string) error {
	f, err := os.Open(csvPath)
	if err != nil {
		return fmt.Errorf("open baselines csv: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	header, err := reader.Read()
	if err != nil {
		return fmt.Errorf("read csv header: %w", err)
	}
	col := map[string]int{}
	for i, name := range header {
		col[name] = i
	}
	required := []string{"community_area_id", "community_area_name", "weekday", "hour", "trip_count", "avg_fare_cents"}
	for _, name := range required {
		if _, ok := col[name]; !ok {
			return fmt.Errorf("csv missing column %q", name)
		}
	}

	tx, err := sqlDB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	areaStmt, err := tx.Prepare(`INSERT OR IGNORE INTO community_areas (id, name) VALUES (?, ?)`)
	if err != nil {
		return err
	}
	defer areaStmt.Close()

	baseStmt, err := tx.Prepare(`
		INSERT INTO hourly_baselines (community_area_id, weekday, hour, trip_count, avg_fare_cents)
		VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer baseStmt.Close()

	rows := 0
	for {
		rec, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read csv row: %w", err)
		}
		id, err := strconv.Atoi(rec[col["community_area_id"]])
		if err != nil {
			return fmt.Errorf("bad community_area_id: %w", err)
		}
		name := rec[col["community_area_name"]]
		weekday, err := strconv.Atoi(rec[col["weekday"]])
		if err != nil {
			return fmt.Errorf("bad weekday: %w", err)
		}
		hour, err := strconv.Atoi(rec[col["hour"]])
		if err != nil {
			return fmt.Errorf("bad hour: %w", err)
		}
		trips, err := strconv.Atoi(rec[col["trip_count"]])
		if err != nil {
			return fmt.Errorf("bad trip_count: %w", err)
		}
		fare, err := strconv.Atoi(rec[col["avg_fare_cents"]])
		if err != nil {
			return fmt.Errorf("bad avg_fare_cents: %w", err)
		}
		if _, err := areaStmt.Exec(id, name); err != nil {
			return fmt.Errorf("insert area %d: %w", id, err)
		}
		if _, err := baseStmt.Exec(id, weekday, hour, trips, fare); err != nil {
			return fmt.Errorf("insert baseline %d %d %d: %w", id, weekday, hour, err)
		}
		rows++
	}
	if rows == 0 {
		return fmt.Errorf("baselines csv had no data rows: %s", csvPath)
	}
	return tx.Commit()
}

// FindBaselinesCSV looks for the shipped Chicago TNP baseline file.
func FindBaselinesCSV() (string, error) {
	candidates := []string{
		filepath.Join("data", "chicago_tnp_baselines.csv"),
		filepath.Join("backend", "data", "chicago_tnp_baselines.csv"),
		filepath.Join("..", "data", "chicago_tnp_baselines.csv"),
		filepath.Join("..", "..", "data", "chicago_tnp_baselines.csv"),
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(dir, "data", "chicago_tnp_baselines.csv"),
			filepath.Join(dir, "..", "data", "chicago_tnp_baselines.csv"),
		)
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs, nil
		}
	}
	return "", fmt.Errorf("chicago_tnp_baselines.csv not found (looked in data/)")
}
