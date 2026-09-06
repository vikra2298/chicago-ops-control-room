package db

import (
	"database/sql"
	"errors"
	"time"
)

// AreaStat is one community area's demand in a weekday/hour window.
// Rank, city average, and gap vs average are computed in SQL.
type AreaStat struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	TripCount    int     `json:"trip_count"`
	AvgFareCents int     `json:"avg_fare_cents"`
	CityAvg      float64 `json:"city_avg"`
	DemandRank   int     `json:"demand_rank"`
	VsCityAvg    float64 `json:"vs_city_avg"`
	DemandRatio  float64 `json:"demand_ratio"`
}

const rankingSQL = `
WITH window_demand AS (
    SELECT
        ca.id,
        ca.name,
        hb.trip_count,
        hb.avg_fare_cents,
        AVG(hb.trip_count * 1.0) OVER () AS city_avg,
        RANK() OVER (ORDER BY hb.trip_count DESC) AS demand_rank
    FROM hourly_baselines hb
    JOIN community_areas ca ON ca.id = hb.community_area_id
    WHERE hb.weekday = ? AND hb.hour = ?
)
SELECT
    id,
    name,
    trip_count,
    avg_fare_cents,
    city_avg,
    demand_rank,
    trip_count - city_avg AS vs_city_avg,
    CAST(trip_count AS REAL) / NULLIF(city_avg, 0) AS demand_ratio
FROM window_demand
ORDER BY demand_rank ASC, name ASC;
`

func (d *DB) RankDemand(weekday, hour int) ([]AreaStat, error) {
	rows, err := d.SQL.Query(rankingSQL, weekday, hour)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AreaStat
	for rows.Next() {
		var a AreaStat
		if err := rows.Scan(
			&a.ID, &a.Name, &a.TripCount, &a.AvgFareCents,
			&a.CityAvg, &a.DemandRank, &a.VsCityAvg, &a.DemandRatio,
		); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (d *DB) SaveWeather(fetchedAt time.Time, tempC, precip float64, code int, label, source string) error {
	_, err := d.SQL.Exec(`
		INSERT INTO weather_snapshots (fetched_at, temp_c, precipitation_mm, weather_code, weather_label, source)
		VALUES (?, ?, ?, ?, ?, ?)`,
		fetchedAt.UTC().Format(time.RFC3339), tempC, precip, code, label, source)
	return err
}

func (d *DB) LatestWeather() (fetchedAt time.Time, tempC, precip float64, code int, label, source string, ok bool, err error) {
	var fetched string
	err = d.SQL.QueryRow(`
		SELECT fetched_at, temp_c, precipitation_mm, weather_code, weather_label, source
		FROM weather_snapshots
		ORDER BY fetched_at DESC
		LIMIT 1`).Scan(&fetched, &tempC, &precip, &code, &label, &source)
	if err != nil {
		ok = false
		if errors.Is(err, sql.ErrNoRows) {
			err = nil
		}
		return
	}
	fetchedAt, err = time.Parse(time.RFC3339, fetched)
	ok = err == nil
	return
}
