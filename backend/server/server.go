package server

import (
	"log/slog"
	"net/http"
	"os"

	"backend/api"
	"backend/ops"
)

// Logger instance (can be set to Text or JSON based on environment)
var logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
	Level: slog.LevelDebug, // Change to LevelInfo to reduce verbosity
}))


// CORS Middleware
func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			logger.Debug("CORS preflight", slog.String("path", r.URL.Path))
			return
		}

		next.ServeHTTP(w, r)
	})
}


// StartServer initializes the HTTP + WebSocket server
func StartServer() {
	// ── 1. configure global logger ────────────────────────────────
	h := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug, // adjust via env/flag if desired
	})
	slog.SetDefault(slog.New(h))

	// ── 2. build OpenAPI‑driven HTTP mux ───────────────────────────
	impl := ops.New()

	openapiMux := api.HandlerWithOptions(
		impl,
		api.StdHTTPServerOptions{BaseURL: ""},
	)

	// ── 3. wrap with CORS (and other middlewares) ─────────────────
	rootHandler := enableCORS(openapiMux)

	// ── 4. serve ──────────────────────────────────────────────────
	addr := "0.0.0.0:8080"
	slog.Info("server starting", "addr", "http://"+addr)

	if err := http.ListenAndServe(addr, rootHandler); err != nil {
		slog.Error("server failed", "err", err)
	}
}