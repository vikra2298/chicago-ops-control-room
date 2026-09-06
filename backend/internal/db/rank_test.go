package db

import (
	"path/filepath"
	"testing"
)

func TestRankDemandOrdersByTripCountAndComputesGap(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	// Replace generated rows for one window so the ranking is obvious.
	_, err = store.SQL.Exec(`DELETE FROM hourly_baselines WHERE weekday = 1 AND hour = 8`)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	fixtures := []struct {
		id    int
		trips int
	}{
		{8, 400},  // Near North Side
		{32, 300}, // Loop
		{54, 40},  // Riverdale
	}
	for _, f := range fixtures {
		_, err = store.SQL.Exec(`
			INSERT INTO hourly_baselines (community_area_id, weekday, hour, trip_count, avg_fare_cents)
			VALUES (?, 1, 8, ?, 1500)`, f.id, f.trips)
		if err != nil {
			t.Fatalf("insert %d: %v", f.id, err)
		}
	}

	ranked, err := store.RankDemand(1, 8)
	if err != nil {
		t.Fatalf("rank: %v", err)
	}
	if len(ranked) < 3 {
		t.Fatalf("expected at least 3 rows, got %d", len(ranked))
	}
	if ranked[0].Name != "Near North Side" || ranked[0].DemandRank != 1 {
		t.Fatalf("want Near North Side rank 1, got %+v", ranked[0])
	}
	if ranked[1].Name != "Loop" || ranked[1].DemandRank != 2 {
		t.Fatalf("want Loop rank 2, got %+v", ranked[1])
	}

	// City average is computed in SQL across every area in that window.
	if ranked[0].CityAvg <= 0 {
		t.Fatalf("city average should be positive, got %v", ranked[0].CityAvg)
	}
	if ranked[0].VsCityAvg <= ranked[1].VsCityAvg {
		t.Fatalf("top area should have the larger gap vs city average")
	}
	if ranked[0].DemandRatio <= 1 {
		t.Fatalf("top area should be above the city average, ratio=%v", ranked[0].DemandRatio)
	}
}
