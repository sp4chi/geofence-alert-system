package httpapi

import (
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"geofencing/internal/geo"
	"geofencing/internal/store"
)

var geofenceCategories = []string{"delivery_zone", "restricted_zone", "toll_zone", "customer_area"}

func isGeofenceCategory(category string) bool {
	for _, c := range geofenceCategories {
		if c == category {
			return true
		}
	}
	return false
}

const (
	maxNameLength        = 200
	maxDescriptionLength = 2000
)

type createGeofenceRequest struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Coordinates [][]float64 `json:"coordinates"`
	Category    string      `json:"category"`
}

type createGeofenceResponse struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type geofenceJSON struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Coordinates [][2]float64 `json:"coordinates"`
	Category    string       `json:"category"`
	CreatedAt   string       `json:"created_at"`
}

type listGeofencesResponse struct {
	Geofences  []geofenceJSON `json:"geofences"`
	TotalCount *int           `json:"total_count,omitempty"` // only with ?limit/offset
}

func (a *API) handleCreateGeofence(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var req createGeofenceRequest
	if err := decodeJSON(w, r, &req); err != nil {
		respondError(w, http.StatusBadRequest, start, err.Error())
		return
	}
	input, problems := req.validate()
	if len(problems) > 0 {
		respondError(w, http.StatusBadRequest, start, "invalid geofence", problems...)
		return
	}

	created, err := a.store.CreateGeofence(r.Context(), input)
	if err != nil {
		respondStoreError(w, r, start, err)
		return
	}
	respond(w, http.StatusCreated, start, createGeofenceResponse{
		ID:     created.ID,
		Name:   created.Name,
		Status: created.Status,
	})
}

// validate checks every field and returns all problems at once, plus the cleaned input.
func (req createGeofenceRequest) validate() (store.NewGeofence, []string) {
	var problems []string

	name := strings.TrimSpace(req.Name)
	description := strings.TrimSpace(req.Description)
	switch {
	case name == "":
		problems = append(problems, "name is required")
	case utf8.RuneCountInString(name) > maxNameLength:
		problems = append(problems, fmt.Sprintf("name must be at most %d characters", maxNameLength))
	}
	if utf8.RuneCountInString(description) > maxDescriptionLength {
		problems = append(problems, fmt.Sprintf("description must be at most %d characters", maxDescriptionLength))
	}

	switch {
	case req.Category == "":
		problems = append(problems, "category is required")
	case !isGeofenceCategory(req.Category):
		problems = append(problems, fmt.Sprintf("category %q is invalid; must be one of: %s",
			req.Category, strings.Join(geofenceCategories, ", ")))
	}

	var ring []geo.Point
	if req.Coordinates == nil {
		problems = append(problems, "coordinates is required")
	} else {
		var ringProblems []string
		ring, ringProblems = geo.ParsePolygon(req.Coordinates)
		problems = append(problems, ringProblems...)
	}

	return store.NewGeofence{Name: name, Description: description, Category: req.Category, Boundary: ring}, problems
}

func (a *API) handleListGeofences(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	category := r.URL.Query().Get("category")
	if category != "" && !isGeofenceCategory(category) {
		respondError(w, http.StatusBadRequest, start, fmt.Sprintf(
			"category %q is invalid; must be one of: %s", category, strings.Join(geofenceCategories, ", ")))
		return
	}

	page, problems := parsePage(r.URL.Query(), 0)
	if len(problems) > 0 {
		respondError(w, http.StatusBadRequest, start, "invalid query parameters", problems...)
		return
	}

	geofences, total, err := a.store.ListGeofences(r.Context(), category, page)
	if err != nil {
		respondStoreError(w, r, start, err)
		return
	}

	out := make([]geofenceJSON, len(geofences))
	for i, g := range geofences {
		out[i] = geofenceJSON{
			ID:          g.ID,
			Name:        g.Name,
			Description: g.Description,
			Coordinates: geo.Pairs(g.Boundary),
			Category:    g.Category,
			CreatedAt:   formatTime(g.CreatedAt),
		}
	}
	respond(w, http.StatusOK, start, listGeofencesResponse{Geofences: out, TotalCount: totalIfPaginated(page, total)})
}
