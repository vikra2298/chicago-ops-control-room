package recommend

import (
	"strings"
	"testing"
	"time"

	"opscontrol/internal/db"
	"opscontrol/internal/weather"
)

func TestDecideRecommendsWhenDemandFarAboveAverageAndWeatherIsBad(t *testing.T) {
	areas := []db.AreaStat{
		{ID: 8, Name: "Near North Side", TripCount: 420, CityAvg: 120, DemandRank: 1, VsCityAvg: 300, DemandRatio: 3.5},
		{ID: 32, Name: "Loop", TripCount: 260, CityAvg: 120, DemandRank: 2, VsCityAvg: 140, DemandRatio: 2.2},
	}
	snap := &weather.Snapshot{
		TempC: 4, PrecipitationMM: 2.4, Code: 61, Label: "Rain", Pressure: "severe", Source: "open-meteo",
		FetchedAt: time.Date(2026, 9, 6, 17, 0, 0, 0, time.UTC),
	}
	now := time.Date(2026, 9, 6, 17, 10, 0, 0, time.UTC) // Sunday afternoon Chicago

	got := Decide(areas, snap, true, now)
	if !got.ActionNeeded {
		t.Fatalf("expected an action, got hold: %+v", got)
	}
	if got.Focus == nil || got.Focus.Name != "Near North Side" {
		t.Fatalf("focus: %+v", got.Focus)
	}
	if got.Severity != "high" {
		t.Fatalf("severe rain + large gap should be high, got %s", got.Severity)
	}
	if got.SuggestedDrivers < 8 {
		t.Fatalf("expected a driver count, got %d", got.SuggestedDrivers)
	}
	if len(got.WhyFacts) < 4 {
		t.Fatalf("expected why facts bullets, got %v", got.WhyFacts)
	}
	joined := strings.Join(got.WhyFacts, " ")
	if !strings.Contains(joined, "Hottest zone now:") || !strings.Contains(joined, "Trips this hour:") {
		t.Fatalf("why facts should include zone and trips: %v", got.WhyFacts)
	}
	if !strings.Contains(joined, "Weather:") || !strings.Contains(joined, "Risk:") {
		t.Fatalf("why facts should include Weather and Risk: %v", got.WhyFacts)
	}
	if !strings.Contains(joined, "Near North Side") {
		t.Fatalf("why facts should name the zone: %v", got.WhyFacts)
	}
}

func TestDecideHoldsWhenDemandIsCloseToAverage(t *testing.T) {
	areas := []db.AreaStat{
		{ID: 8, Name: "Near North Side", TripCount: 130, CityAvg: 120, DemandRank: 1, VsCityAvg: 10, DemandRatio: 1.08},
	}
	snap := &weather.Snapshot{TempC: 21, PrecipitationMM: 0, Code: 0, Label: "Clear", Pressure: "normal"}
	now := time.Date(2026, 9, 8, 15, 0, 0, 0, time.UTC)

	got := Decide(areas, snap, true, now)
	if got.ActionNeeded {
		t.Fatalf("flat demand should hold, got %+v", got)
	}
	if got.Severity != "hold" {
		t.Fatalf("severity=%s", got.Severity)
	}
	if !strings.Contains(strings.ToLower(got.Headline), "no supply") {
		t.Fatalf("headline: %s", got.Headline)
	}
	joined := strings.Join(got.WhyFacts, " ")
	if !strings.Contains(joined, "below intervention") || !strings.Contains(joined, "Risk:") {
		t.Fatalf("hold why facts: %v", got.WhyFacts)
	}
}
