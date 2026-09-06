package main

import (
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "time/tzdata"

	"opscontrol/internal/api"
	"opscontrol/internal/db"
	"opscontrol/internal/weather"
)

func main() {
	addr := listenAddr()
	dbPath := env("DB_PATH", defaultDBPath())

	store, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer store.Close()

	server := api.New(store, weather.NewClient())
	mux := http.NewServeMux()
	server.Routes(mux)

	if dist := findFrontend(); dist != "" {
		log.Printf("serving frontend from %s", dist)
		mux.Handle("/", spaHandler(dist))
	} else {
		log.Printf("frontend build not found; API only. Run the Vite app or build frontend/dist")
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "API is up. Build the frontend (npm run build) or use docker compose.", http.StatusNotFound)
		})
	}

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           api.CORS(logging(mux)),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("ops control room listening on %s (db=%s)", addr, dbPath)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func defaultDBPath() string {
	if cwd, err := os.Getwd(); err == nil {
		return filepath.Join(cwd, "data", "ops.db")
	}
	return filepath.Join("data", "ops.db")
}

func findFrontend() string {
	candidates := []string{
		"frontend/dist",
		filepath.Join("..", "frontend", "dist"),
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(dir, "frontend", "dist"),
			filepath.Join(dir, "..", "frontend", "dist"),
		)
	}
	for _, c := range candidates {
		if st, err := os.Stat(filepath.Join(c, "index.html")); err == nil && !st.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	return ""
}

func spaHandler(dist string) http.Handler {
	root := http.Dir(dist)
	fileServer := http.FileServer(root)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		requestPath := strings.TrimPrefix(r.URL.Path, "/")
		if requestPath == "" {
			fileServer.ServeHTTP(w, r)
			return
		}
		if _, err := fs.Stat(os.DirFS(dist), requestPath); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(dist, "index.html"))
	})
}

func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Truncate(time.Millisecond))
	})
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// listenAddr prefers ADDR, then PORT (Railway/Render/Fly), then :8080.
func listenAddr() string {
	if v := os.Getenv("ADDR"); v != "" {
		return v
	}
	if p := os.Getenv("PORT"); p != "" {
		if strings.HasPrefix(p, ":") {
			return p
		}
		return ":" + p
	}
	return ":8080"
}
