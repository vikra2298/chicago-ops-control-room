package db

import (
	"path/filepath"
	"testing"
)

func TestSeedFromCSVLoadsBaselines(t *testing.T) {
	dir := t.TempDir()
	csvPath, err := FindBaselinesCSV()
	if err != nil {
		t.Fatalf("find csv: %v", err)
	}

	// Force empty DB at a unique path, then load CSV explicitly.
	dbPath := filepath.Join(dir, "ops.db")
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	var areas, baselines int
	if err := store.SQL.QueryRow(`SELECT COUNT(*) FROM community_areas`).Scan(&areas); err != nil {
		t.Fatal(err)
	}
	if err := store.SQL.QueryRow(`SELECT COUNT(*) FROM hourly_baselines`).Scan(&baselines); err != nil {
		t.Fatal(err)
	}
	if areas != 77 {
		t.Fatalf("areas=%d want 77 (from %s)", areas, csvPath)
	}
	if baselines != 77*7*24 {
		t.Fatalf("baselines=%d want %d", baselines, 77*7*24)
	}
}
