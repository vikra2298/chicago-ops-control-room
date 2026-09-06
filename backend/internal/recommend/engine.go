package recommend

import (
	"fmt"
	"math"
	"strings"
	"time"

	"opscontrol/internal/db"
	"opscontrol/internal/weather"
)

type Window struct {
	Weekday     int    `json:"weekday"`
	WeekdayName string `json:"weekday_name"`
	Hour        int    `json:"hour"`
	Label       string `json:"label"`
	Timezone    string `json:"timezone"`
}

type WeatherInfo struct {
	TempC           float64 `json:"temp_c"`
	PrecipitationMM float64 `json:"precipitation_mm"`
	WindKmh         float64 `json:"wind_kmh"`
	Code            int     `json:"weather_code"`
	Label           string  `json:"label"`
	Pressure        string  `json:"pressure"`
	Source          string  `json:"source"`
	FetchedAt       string  `json:"fetched_at"`
}

type Decision struct {
	ActionNeeded     bool          `json:"action_needed"`
	Severity         string        `json:"severity"`
	Headline         string        `json:"headline"`
	Action           string        `json:"action"`
	Why              string        `json:"why"`
	WhyFacts         []string      `json:"why_facts"`
	Focus            *db.AreaStat  `json:"focus"`
	Areas            []db.AreaStat `json:"areas"`
	CityAvgTrips     float64       `json:"city_avg_trips"`
	ThresholdUsed    float64       `json:"threshold_used"`
	SuggestedDrivers int           `json:"suggested_drivers"`
	Window           Window        `json:"window"`
	Weather          *WeatherInfo  `json:"weather"`
	WeatherAvailable bool          `json:"weather_available"`
	WeatherNote      string        `json:"weather_note"`
	Uncertainty      []string      `json:"uncertainty"`
	GeneratedAt      string        `json:"generated_at"`
}

var weekdayNames = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

func Decide(areas []db.AreaStat, snap *weather.Snapshot, weatherOK bool, now time.Time) Decision {
	loc, err := time.LoadLocation("America/Chicago")
	if err != nil {
		loc = now.Location()
	}
	local := now.In(loc)
	weekday := int(local.Weekday())
	hour := local.Hour()

	decision := Decision{
		Window: Window{
			Weekday:     weekday,
			WeekdayName: weekdayNames[weekday],
			Hour:        hour,
			Label:       fmt.Sprintf("%s %02d:00-%02d:00", weekdayNames[weekday], hour, (hour+1)%24),
			Timezone:    "America/Chicago",
		},
		GeneratedAt: local.Format(time.RFC3339),
		Areas:       topAreas(areas, 8),
	}
	if len(areas) > 0 {
		decision.CityAvgTrips = areas[0].CityAvg
		focus := areas[0]
		decision.Focus = &focus
	}

	if weatherOK && snap != nil {
		decision.WeatherAvailable = true
		decision.Weather = &WeatherInfo{
			TempC:           snap.TempC,
			PrecipitationMM: snap.PrecipitationMM,
			WindKmh:         snap.WindKmh,
			Code:            snap.Code,
			Label:           snap.Label,
			Pressure:        snap.Pressure,
			Source:          snap.Source,
			FetchedAt:       snap.FetchedAt.In(loc).Format(time.RFC3339),
		}
	} else {
		decision.WeatherNote = "Live weather was unavailable. The recommendation uses the historical demand baseline only."
	}

	threshold := 1.45
	pressure := "normal"
	if decision.Weather != nil {
		pressure = decision.Weather.Pressure
		switch pressure {
		case "severe":
			threshold = 1.18
		case "elevated":
			threshold = 1.28
		}
	}
	decision.ThresholdUsed = threshold

	if decision.Focus == nil || decision.Focus.DemandRatio < threshold {
		decision.ActionNeeded = false
		decision.Severity = "hold"
		decision.Headline = "No supply intervention needed this hour"
		decision.Action = "Hold driver positioning. Keep watching the usual downtown and airport corridors."
		decision.WhyFacts = holdWhyFacts(decision.Focus, decision.Weather, decision.CityAvgTrips)
		decision.Why = strings.Join(decision.WhyFacts, " | ")
		decision.Uncertainty = uncertainty(decision, false)
		return decision
	}

	drivers := suggestedDrivers(decision.Focus)
	decision.ActionNeeded = true
	decision.SuggestedDrivers = drivers
	if decision.Focus.DemandRatio >= 2.2 || pressure == "severe" {
		decision.Severity = "high"
	} else {
		decision.Severity = "moderate"
	}
	decision.Headline = fmt.Sprintf("Reposition %d idle drivers to %s for the next 60 minutes", drivers, decision.Focus.Name)
	decision.Action = fmt.Sprintf("Move idle supply into %s before this hour-of-week demand window fades.", decision.Focus.Name)
	decision.WhyFacts = actWhyFacts(decision.Focus, decision.Weather, decision.CityAvgTrips)
	decision.Why = strings.Join(decision.WhyFacts, " | ")
	decision.Uncertainty = uncertainty(decision, true)
	return decision
}

func suggestedDrivers(focus *db.AreaStat) int {
	if focus == nil {
		return 0
	}
	n := int(math.Round(focus.VsCityAvg / 18.0))
	if n < 8 {
		n = 8
	}
	if n > 28 {
		n = 28
	}
	return n
}

func topAreas(areas []db.AreaStat, n int) []db.AreaStat {
	if len(areas) <= n {
		return areas
	}
	return areas[:n]
}

func actWhyFacts(focus *db.AreaStat, w *WeatherInfo, cityAvg float64) []string {
	return []string{
		fmt.Sprintf("Hottest zone now: %s (#%d)", focus.Name, focus.DemandRank),
		fmt.Sprintf("Trips this hour: %d (monitored-area average is ~%.0f)", focus.TripCount, cityAvg),
		weatherFact(w, true),
		"Risk: wait times stretch if supply stays flat",
	}
}

func holdWhyFacts(focus *db.AreaStat, w *WeatherInfo, cityAvg float64) []string {
	if focus == nil {
		return []string{
			"Hottest zone now: none for this hour",
			"Trips this hour: unavailable",
			weatherFact(w, false),
			"Risk: low - no strong hotspot right now",
		}
	}
	return []string{
		fmt.Sprintf("Hottest zone now: %s (#%d)", focus.Name, focus.DemandRank),
		fmt.Sprintf("Trips this hour: %d (monitored-area average is ~%.0f) - below intervention threshold", focus.TripCount, cityAvg),
		weatherFact(w, false),
		"Risk: low - no strong hotspot right now",
	}
}

func weatherFact(w *WeatherInfo, acting bool) string {
	if w == nil {
		return "Weather: unavailable"
	}
	base := fmt.Sprintf("Weather: %s, %.0f\u00b0C", w.Label, w.TempC)
	switch w.Pressure {
	case "severe":
		return base + " - weather adjustment applied"
	case "elevated":
		return base + " - weather adjustment applied"
	default:
		return base + " - no weather adjustment applied"
	}
}

func uncertainty(d Decision, acting bool) []string {
	items := []string{
		"Historical demand is an hour-of-week pattern, not live trips — events can shift the hotspot.",
		"Community areas are large; the real spike may be a few blocks.",
		"Weather is city-wide; local rain can differ.",
		"Driver count is a simple estimate, not an optimized quota.",
		"If demand is soft today (holiday / remote Friday), extra supply may sit idle.",
	}
	if !d.WeatherAvailable {
		items = append([]string{"Live weather unavailable — decision uses historical demand only."}, items...)
	}
	_ = acting
	return items
}
