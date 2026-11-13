package httpapi

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"geofencing/internal/geo"
	"geofencing/internal/tracking"
)

// maxClockSkew bounds how far in the future a timestamp may be. Without it, one point from a
// device with a wrong clock would make every later real point look "late" and freeze the vehicle.
const maxClockSkew = 5 * time.Minute

type updateLocationRequest struct {
	VehicleID string `json:"vehicle_id"`
	// Pointers distinguish "missing" from a legitimate 0 (the equator / prime meridian).
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
	Timestamp string   `json:"timestamp"`
}

type insideGeofenceJSON struct {
	GeofenceID   string `json:"geofence_id"`
	GeofenceName string `json:"geofence_name"`
	Status       string `json:"status"`
}

type updateLocationResponse struct {
	VehicleID        string               `json:"vehicle_id"`
	LocationUpdated  bool                 `json:"location_updated"`
	CurrentGeofences []insideGeofenceJSON `json:"current_geofences"`
}

func (a *API) handleUpdateLocation(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var req updateLocationRequest
	if err := decodeJSON(w, r, &req); err != nil {
		respondError(w, http.StatusBadRequest, start, err.Error())
		return
	}
	update, problems := req.validate(time.Now())
	if len(problems) > 0 {
		respondError(w, http.StatusBadRequest, start, "invalid location update", problems...)
		return
	}

	result, err := a.tracker.UpdateLocation(r.Context(), update)
	if err != nil {
		respondStoreError(w, r, start, err) // unknown vehicle → 404
		return
	}

	inside := make([]insideGeofenceJSON, len(result.Current))
	for i, g := range result.Current {
		inside[i] = insideGeofenceJSON{GeofenceID: g.ID, GeofenceName: g.Name, Status: "inside"}
	}
	// location_updated is true whenever the point was stored. A late point is stored in history
	// but leaves the current position alone; current_geofences then shows the unchanged state.
	respond(w, http.StatusOK, start, updateLocationResponse{
		VehicleID:        update.VehicleID,
		LocationUpdated:  true,
		CurrentGeofences: inside,
	})
}

func (req updateLocationRequest) validate(now time.Time) (tracking.Update, []string) {
	var (
		problems []string
		update   = tracking.Update{VehicleID: strings.TrimSpace(req.VehicleID)}
	)

	if update.VehicleID == "" {
		problems = append(problems, "vehicle_id is required")
	}

	if req.Latitude == nil {
		problems = append(problems, "latitude is required")
	}
	if req.Longitude == nil {
		problems = append(problems, "longitude is required")
	}
	if req.Latitude != nil && req.Longitude != nil {
		update.Position = geo.Point{Lat: *req.Latitude, Lng: *req.Longitude}
		problems = append(problems, geo.ValidatePoint(update.Position)...)
	}

	switch recordedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(req.Timestamp)); {
	case req.Timestamp == "":
		problems = append(problems, "timestamp is required")
	case err != nil:
		problems = append(problems, fmt.Sprintf(
			"timestamp %q must be ISO 8601 / RFC 3339 with a timezone, e.g. 2025-01-15T10:35:00Z", req.Timestamp))
	case recordedAt.After(now.Add(maxClockSkew)):
		problems = append(problems, "timestamp is in the future (more than 5 minutes ahead of server time)")
	default:
		update.RecordedAt = recordedAt
	}

	return update, problems
}

type locationJSON struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Timestamp string  `json:"timestamp"`
}

type geofenceMembershipJSON struct {
	GeofenceID   string `json:"geofence_id"`
	GeofenceName string `json:"geofence_name"`
	Category     string `json:"category"`
}

type vehicleLocationResponse struct {
	VehicleID     string `json:"vehicle_id"`
	VehicleNumber string `json:"vehicle_number"`
	// null until the vehicle's first location update.
	CurrentLocation  *locationJSON            `json:"current_location"`
	CurrentGeofences []geofenceMembershipJSON `json:"current_geofences"`
}

func (a *API) handleGetVehicleLocation(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	vehicleID := r.PathValue("vehicle_id")
	location, err := a.store.GetVehicleLocation(r.Context(), vehicleID)
	if err != nil {
		respondStoreError(w, r, start, err)
		return
	}

	out := vehicleLocationResponse{
		VehicleID:        location.VehicleID,
		VehicleNumber:    location.VehicleNumber,
		CurrentGeofences: make([]geofenceMembershipJSON, len(location.Geofences)),
	}
	if location.Location != nil && location.LastSeenAt != nil {
		out.CurrentLocation = &locationJSON{
			Latitude:  location.Location.Lat,
			Longitude: location.Location.Lng,
			Timestamp: formatTime(*location.LastSeenAt),
		}
	}
	for i, g := range location.Geofences {
		out.CurrentGeofences[i] = geofenceMembershipJSON{GeofenceID: g.ID, GeofenceName: g.Name, Category: g.Category}
	}
	respond(w, http.StatusOK, start, out)
}
