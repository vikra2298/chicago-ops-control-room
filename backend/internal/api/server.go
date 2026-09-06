package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"opscontrol/internal/db"
	"opscontrol/internal/recommend"
	"opscontrol/internal/weather"
)

type Server struct {
	DB      *db.DB
	Weather *weather.Client
	Now     func() time.Time
}

func New(store *db.DB, weatherClient *weather.Client) *Server {
	return &Server{
		DB:      store,
		Weather: weatherClient,
		Now:     time.Now,
	}
}

func (s *Server) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/recommendation", s.handleRecommendation)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	var areas int
	err := s.DB.SQL.QueryRow(`SELECT COUNT(*) FROM community_areas`).Scan(&areas)
	status := "ok"
	if err != nil {
		status = "degraded"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": status,
		"areas":  areas,
		"time":   s.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleRecommendation(w http.ResponseWriter, r *http.Request) {
	now := s.Now()
	loc, err := time.LoadLocation("America/Chicago")
	if err != nil {
		loc = time.UTC
	}
	local := now.In(loc)

	areas, err := s.DB.RankDemand(int(local.Weekday()), local.Hour())
	if err != nil {
		http.Error(w, "ranking query failed", http.StatusInternalServerError)
		log.Printf("rank: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var snap *weather.Snapshot
	weatherOK := false
	live, err := s.Weather.Current(ctx)
	if err == nil {
		weatherOK = true
		liveCopy := live
		snap = &liveCopy
		if saveErr := s.DB.SaveWeather(live.FetchedAt, live.TempC, live.PrecipitationMM, live.Code, live.Label, live.Source); saveErr != nil {
			log.Printf("save weather: %v", saveErr)
		}
	} else {
		log.Printf("weather live fetch failed: %v", err)
		if cached, ok := s.cachedWeather(30 * time.Minute); ok {
			weatherOK = true
			snap = cached
			log.Printf("using cached weather from %s", cached.FetchedAt)
		}
	}

	decision := recommend.Decide(areas, snap, weatherOK, now)
	if snap != nil && snap.Source == "open-meteo-cache" {
		decision.WeatherNote = "Live weather timed out. Using the latest stored observation from the last 30 minutes."
	}
	writeJSON(w, http.StatusOK, decision)
}

func (s *Server) cachedWeather(maxAge time.Duration) (*weather.Snapshot, bool) {
	fetchedAt, tempC, precip, code, label, source, ok, err := s.DB.LatestWeather()
	if err != nil || !ok {
		if err != nil && err != sql.ErrNoRows {
			log.Printf("latest weather: %v", err)
		}
		return nil, false
	}
	if time.Since(fetchedAt) > maxAge {
		return nil, false
	}
	return &weather.Snapshot{
		FetchedAt:       fetchedAt,
		TempC:           tempC,
		PrecipitationMM: precip,
		Code:            code,
		Label:           label,
		Pressure:        weather.Pressure(code, precip, tempC),
		Source:          source + "-cache",
	}, true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
