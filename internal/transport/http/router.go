package http

import (
	"net/http"

	"github.com/EyalShahaf/temporal-seats/internal/config"
	"go.temporal.io/sdk/client"
)

// NewRouter creates and configures the main HTTP router for the service.
func NewRouter(cfg config.Config, temporal client.Client) http.Handler {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","service":"temporal-seats-api"}`))
	})

	orderHandler := NewOrderHandler(temporal)
	orderHandler.attachOrderRoutes(mux)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(204)
			return
		}
		mux.ServeHTTP(w, r)
	})
}
