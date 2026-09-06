package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const DefaultURL = "https://api.open-meteo.com/v1/forecast?latitude=41.8781&longitude=-87.6298&current=temperature_2m,precipitation,weather_code,wind_speed_10m&timezone=America/Chicago"

type Snapshot struct {
	FetchedAt       time.Time
	TempC           float64
	PrecipitationMM float64
	WindKmh         float64
	Code            int
	Label           string
	Pressure        string
	Source          string
}

type Client struct {
	HTTP    *http.Client
	URL     string
	Source  string
	Timeout time.Duration
}

func NewClient() *Client {
	return &Client{
		HTTP:    &http.Client{Timeout: 4 * time.Second},
		URL:     DefaultURL,
		Source:  "open-meteo",
		Timeout: 4 * time.Second,
	}
}

type openMeteoResponse struct {
	Current struct {
		Temperature   float64 `json:"temperature_2m"`
		Precipitation float64 `json:"precipitation"`
		WeatherCode   int     `json:"weather_code"`
		WindSpeed     float64 `json:"wind_speed_10m"`
	} `json:"current"`
}

func (c *Client) Current(ctx context.Context) (Snapshot, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL, nil)
	if err != nil {
		return Snapshot{}, err
	}
	req.Header.Set("Accept", "application/json")

	res, err := c.HTTP.Do(req)
	if err != nil {
		return Snapshot{}, fmt.Errorf("weather request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return Snapshot{}, fmt.Errorf("weather status %d", res.StatusCode)
	}

	var payload openMeteoResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return Snapshot{}, fmt.Errorf("weather decode: %w", err)
	}

	code := payload.Current.WeatherCode
	return Snapshot{
		FetchedAt:       time.Now().UTC(),
		TempC:           payload.Current.Temperature,
		PrecipitationMM: payload.Current.Precipitation,
		WindKmh:         payload.Current.WindSpeed,
		Code:            code,
		Label:           Label(code),
		Pressure:        Pressure(code, payload.Current.Precipitation, payload.Current.Temperature),
		Source:          c.Source,
	}, nil
}
