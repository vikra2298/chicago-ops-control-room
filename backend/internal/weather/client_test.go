package weather

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCurrentReturnsErrorWhenAPIFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream down", http.StatusBadGateway)
	}))
	t.Cleanup(srv.Close)

	client := &Client{
		HTTP:   srv.Client(),
		URL:    srv.URL,
		Source: "test",
	}

	_, err := client.Current(context.Background())
	if err == nil {
		t.Fatal("expected an error when Open-Meteo returns 502")
	}
}

func TestCurrentParsesLivePayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"current":{"temperature_2m":3.2,"precipitation":1.1,"weather_code":61,"wind_speed_10m":22}}`))
	}))
	t.Cleanup(srv.Close)

	client := &Client{HTTP: srv.Client(), URL: srv.URL, Source: "test"}
	snap, err := client.Current(context.Background())
	if err != nil {
		t.Fatalf("current: %v", err)
	}
	if snap.Label != "Rain" {
		t.Fatalf("label=%s", snap.Label)
	}
	if snap.Pressure != "elevated" && snap.Pressure != "severe" {
		t.Fatalf("rain should raise pressure, got %s", snap.Pressure)
	}
	if snap.TempC != 3.2 {
		t.Fatalf("temp=%v", snap.TempC)
	}
}

func TestCurrentTimesOut(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"current":{}}`))
	}))
	t.Cleanup(srv.Close)

	client := &Client{
		HTTP: &http.Client{Timeout: 50 * time.Millisecond},
		URL:  srv.URL,
	}
	_, err := client.Current(context.Background())
	if err == nil {
		t.Fatal("expected timeout error")
	}
}
