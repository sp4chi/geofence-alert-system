package httpapi

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"geofencing/internal/store"
)

const (
	defaultHistoryLimit = 50
	maxHistoryLimit     = 500
)

type violationJSON struct {
	ID            string  `json:"id"`
	VehicleID     string  `json:"vehicle_id"`
	VehicleNumber string  `json:"vehicle_number"`
	GeofenceID    string  `json:"geofence_id"`
	GeofenceName  string  `json:"geofence_name"`
	EventType     string  `json:"event_type"`
	Latitude      float64 `json:"latitude"`
	Longitude     float64 `json:"longitude"`
	Timestamp     string  `json:"timestamp"`
}

type violationHistoryResponse struct {
	Violations []violationJSON `json:"violations"`
	TotalCount int             `json:"total_count"`
}

func (a *API) handleViolationHistory(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	filter, problems := parseHistoryFilter(r.URL.Query())
	if len(problems) > 0 {
		respondError(w, http.StatusBadRequest, start, "invalid query parameters", problems...)
		return
	}

	violations, total, err := a.store.ListViolations(r.Context(), filter)
	if err != nil {
		respondStoreError(w, r, start, err)
		return
	}

	out := make([]violationJSON, len(violations))
	for i, v := range violations {
		out[i] = violationJSON{
			ID:            v.ID,
			VehicleID:     v.VehicleID,
			VehicleNumber: v.VehicleNumber,
			GeofenceID:    v.GeofenceID,
			GeofenceName:  v.GeofenceName,
			EventType:     v.EventType,
			Latitude:      v.Location.Lat,
			Longitude:     v.Location.Lng,
			Timestamp:     formatTime(v.OccurredAt),
		}
	}
	respond(w, http.StatusOK, start, violationHistoryResponse{Violations: out, TotalCount: total})
}

// parseHistoryFilter reads the query string. limit defaults to 50; values above 500 are clamped
// to 500 rather than rejected (documented decision). offset is our addition for pagination.
func parseHistoryFilter(query url.Values) (store.HistoryFilter, []string) {
	var problems []string
	filter := store.HistoryFilter{
		VehicleID:  strings.TrimSpace(query.Get("vehicle_id")),
		GeofenceID: strings.TrimSpace(query.Get("geofence_id")),
		Limit:      defaultHistoryLimit,
	}

	parseDate := func(name string) *time.Time {
		raw := strings.TrimSpace(query.Get(name))
		if raw == "" {
			return nil
		}
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s %q must be ISO 8601 / RFC 3339, e.g. 2025-01-01T00:00:00Z", name, raw))
			return nil
		}
		return &t
	}
	filter.Start = parseDate("start_date")
	filter.End = parseDate("end_date")
	if filter.Start != nil && filter.End != nil && filter.Start.After(*filter.End) {
		problems = append(problems, "start_date must be before or equal to end_date")
	}

	if raw := query.Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		switch {
		case err != nil || limit < 1:
			problems = append(problems, fmt.Sprintf("limit %q must be a positive integer (max %d)", raw, maxHistoryLimit))
		case limit > maxHistoryLimit:
			filter.Limit = maxHistoryLimit
		default:
			filter.Limit = limit
		}
	}

	if raw := query.Get("offset"); raw != "" {
		offset, err := strconv.Atoi(raw)
		if err != nil || offset < 0 {
			problems = append(problems, fmt.Sprintf("offset %q must be a non-negative integer", raw))
		} else {
			filter.Offset = offset
		}
	}

	return filter, problems
}
