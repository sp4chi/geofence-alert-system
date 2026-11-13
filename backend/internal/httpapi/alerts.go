package httpapi

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"geofencing/internal/store"
)

var alertEventTypes = []string{"entry", "exit", "both"}

type configureAlertRequest struct {
	GeofenceID string `json:"geofence_id"`
	VehicleID  string `json:"vehicle_id"` // optional; omitted or "" = all vehicles
	EventType  string `json:"event_type"`
}

// vehicle_id is a pointer so "all vehicles" is returned as JSON null.
type configureAlertResponse struct {
	AlertID    string  `json:"alert_id"`
	GeofenceID string  `json:"geofence_id"`
	VehicleID  *string `json:"vehicle_id"`
	EventType  string  `json:"event_type"`
	Status     string  `json:"status"`
}

type alertRuleJSON struct {
	AlertID       string  `json:"alert_id"`
	GeofenceID    string  `json:"geofence_id"`
	GeofenceName  string  `json:"geofence_name"`
	VehicleID     *string `json:"vehicle_id"`
	VehicleNumber *string `json:"vehicle_number"`
	EventType     string  `json:"event_type"`
	Status        string  `json:"status"`
	CreatedAt     string  `json:"created_at"`
}

type listAlertsResponse struct {
	Alerts []alertRuleJSON `json:"alerts"`
}

func (a *API) handleConfigureAlert(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var req configureAlertRequest
	if err := decodeJSON(w, r, &req); err != nil {
		respondError(w, http.StatusBadRequest, start, err.Error())
		return
	}
	input, problems := req.validate()
	if len(problems) > 0 {
		respondError(w, http.StatusBadRequest, start, "invalid alert rule", problems...)
		return
	}

	rule, err := a.store.CreateAlertRule(r.Context(), input)
	if err != nil {
		respondStoreError(w, r, start, err) // unknown geofence/vehicle → 404, identical rule → 409
		return
	}
	respond(w, http.StatusCreated, start, configureAlertResponse{
		AlertID:    rule.ID,
		GeofenceID: rule.GeofenceID,
		VehicleID:  rule.VehicleID,
		EventType:  rule.EventType,
		Status:     rule.Status,
	})
}

func (req configureAlertRequest) validate() (store.NewAlertRule, []string) {
	var problems []string
	input := store.NewAlertRule{
		GeofenceID: strings.TrimSpace(req.GeofenceID),
		EventType:  req.EventType,
	}

	if input.GeofenceID == "" {
		problems = append(problems, "geofence_id is required")
	}
	if vehicleID := strings.TrimSpace(req.VehicleID); vehicleID != "" {
		input.VehicleID = &vehicleID
	}

	switch {
	case req.EventType == "":
		problems = append(problems, "event_type is required")
	case !contains(alertEventTypes, req.EventType):
		problems = append(problems, fmt.Sprintf("event_type %q is invalid; must be one of: %s",
			req.EventType, strings.Join(alertEventTypes, ", ")))
	}
	return input, problems
}

func (a *API) handleListAlerts(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	query := r.URL.Query()
	rules, err := a.store.ListAlertRules(r.Context(),
		strings.TrimSpace(query.Get("geofence_id")),
		strings.TrimSpace(query.Get("vehicle_id")))
	if err != nil {
		respondStoreError(w, r, start, err)
		return
	}

	out := make([]alertRuleJSON, len(rules))
	for i, rule := range rules {
		out[i] = alertRuleJSON{
			AlertID:       rule.ID,
			GeofenceID:    rule.GeofenceID,
			GeofenceName:  rule.GeofenceName,
			VehicleID:     rule.VehicleID,
			VehicleNumber: rule.VehicleNumber,
			EventType:     rule.EventType,
			Status:        rule.Status,
			CreatedAt:     formatTime(rule.CreatedAt),
		}
	}
	respond(w, http.StatusOK, start, listAlertsResponse{Alerts: out})
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
