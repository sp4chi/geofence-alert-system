package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"geofencing/internal/store"
)

// respondStoreError is the single place where domain errors become HTTP status codes:
// invalid geometry → 400, not found → 404, duplicate → 409, anything else → 500 (details logged, not leaked).
func respondStoreError(w http.ResponseWriter, r *http.Request, start time.Time, err error) {
	var geometryErr *store.InvalidGeometryError

	switch {
	case errors.As(err, &geometryErr):
		respondError(w, http.StatusBadRequest, start, "invalid geofence", describeGeometryProblem(geometryErr.Reason))
	case errors.Is(err, store.ErrNotFound):
		respondError(w, http.StatusNotFound, start, store.ClientMessage(err))
	case errors.Is(err, store.ErrDuplicate):
		respondError(w, http.StatusConflict, start, store.ClientMessage(err))
	case errors.Is(err, context.Canceled):
		// The client disconnected; nobody is listening, but keep the contract anyway.
		respondError(w, http.StatusServiceUnavailable, start, "request cancelled")
	default:
		slog.Error("request failed", "method", r.Method, "path", r.URL.Path, "err", err)
		respondError(w, http.StatusInternalServerError, start, "internal server error")
	}
}

// describeGeometryProblem turns a PostGIS ST_IsValidReason into plain language.
func describeGeometryProblem(reason string) string {
	switch {
	case strings.HasPrefix(reason, "Self-intersection"), strings.HasPrefix(reason, "Ring Self-intersection"):
		return "polygon edges cross each other (self-intersecting shape)"
	case strings.HasPrefix(reason, "Too few points"):
		return "polygon has no area (points are in a straight line)"
	default:
		return "polygon shape is invalid: " + reason
	}
}
