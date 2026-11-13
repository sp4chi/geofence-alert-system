// Package httpapi is the HTTP layer: routing, handlers, middleware and JSON responses.
// Handlers stay thin: parse → validate → call the store/service → respond.
package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"geofencing/internal/store"
	"geofencing/internal/tracking"
)

// Pinger is the slice of the database the health check needs; an interface keeps it testable without Postgres.
type Pinger interface {
	Ping(ctx context.Context) error
}

type Options struct {
	Health      Pinger
	Store       *store.Store
	Tracker     *tracking.Service
	CORSOrigins []string
}

type API struct {
	health  Pinger
	store   *store.Store
	tracker *tracking.Service
}

// NewRouter wires every route. Go 1.22+ patterns ("GET /path/{id}") give method matching and path params.
func NewRouter(opts Options) http.Handler {
	api := &API{health: opts.Health, store: opts.Store, tracker: opts.Tracker}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", api.handleHealth)

	mux.HandleFunc("POST /geofences", api.handleCreateGeofence)
	mux.HandleFunc("GET /geofences", api.handleListGeofences)

	mux.HandleFunc("POST /vehicles", api.handleCreateVehicle)
	mux.HandleFunc("GET /vehicles", api.handleListVehicles)
	mux.HandleFunc("POST /vehicles/location", api.handleUpdateLocation)
	mux.HandleFunc("GET /vehicles/location/{vehicle_id}", api.handleGetVehicleLocation)

	mux.HandleFunc("GET /violations/history", api.handleViolationHistory)

	// Catch-all so unknown paths still get a JSON body with time_ns instead of the stdlib text 404.
	mux.HandleFunc("/", handleNotFound)

	// Outermost first: log sees the final status, including 500s produced by recoverPanics.
	return chain(mux, logRequests, recoverPanics, cors(opts.CORSOrigins))
}

type healthResponse struct {
	Status   string `json:"status"`
	Database string `json:"database"`
}

func (a *API) handleHealth(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := a.health.Ping(ctx); err != nil {
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
