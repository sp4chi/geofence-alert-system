// Package httpapi is the HTTP layer: routing, handlers, middleware and JSON responses.
// Handlers stay thin: parse → validate → call a service → respond.
package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// Pinger is the slice of the database the health check needs; an interface keeps it testable without Postgres.
type Pinger interface {
	Ping(ctx context.Context) error
}

type API struct {
	db Pinger
}

// NewRouter wires every route. Go 1.22+ patterns ("GET /path/{id}") give method matching and path params.
func NewRouter(db Pinger) http.Handler {
	api := &API{db: db}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", api.handleHealth)

	// Catch-all so unknown paths still get a JSON body with time_ns instead of the stdlib text 404.
	mux.HandleFunc("/", handleNotFound)

	return mux
}

type healthResponse struct {
	Status   string `json:"status"`
	Database string `json:"database"`
}

func (a *API) handleHealth(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := a.db.Ping(ctx); err != nil {
		slog.Warn("health check: database ping failed", "err", err)
		respondError(w, http.StatusServiceUnavailable, start, "database unavailable")
		return
	}
	respond(w, http.StatusOK, start, healthResponse{Status: "ok", Database: "ok"})
}

func handleNotFound(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	respondError(w, http.StatusNotFound, start, "route not found: "+r.Method+" "+r.URL.Path)
}
