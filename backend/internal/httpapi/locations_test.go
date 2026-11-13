package httpapi

import (
	"net/url"
	"testing"
	"time"
)

func float(v float64) *float64 { return &v }

func TestUpdateLocationValidate(t *testing.T) {
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	valid := updateLocationRequest{VehicleID: "veh_1", Latitude: float(37.7849), Longitude: float(-122.4194), Timestamp: "2025-01-15T10:35:00Z"}

	tests := []struct {
		name         string
		mutate       func(*updateLocationRequest)
		wantProblems []string
	}{
		{name: "valid", mutate: func(*updateLocationRequest) {}},
		{name: "zero lat/lng is a real place", mutate: func(r *updateLocationRequest) { r.Latitude, r.Longitude = float(0), float(0) }},
		{name: "offset timezone", mutate: func(r *updateLocationRequest) { r.Timestamp = "2025-01-15T16:05:00+05:30" }},
		{name: "fractional seconds", mutate: func(r *updateLocationRequest) { r.Timestamp = "2025-01-15T10:35:00.123Z" }},
		{
			name:         "everything missing",
			mutate:       func(r *updateLocationRequest) { *r = updateLocationRequest{} },
			wantProblems: []string{"vehicle_id is required", "latitude is required", "longitude is required", "timestamp is required"},
		},
		{name: "latitude out of range", mutate: func(r *updateLocationRequest) { r.Latitude = float(-91) }, wantProblems: []string{"latitude -91 is out of range"}},
		{name: "longitude out of range", mutate: func(r *updateLocationRequest) { r.Longitude = float(181) }, wantProblems: []string{"longitude 181 is out of range"}},
		{name: "no timezone", mutate: func(r *updateLocationRequest) { r.Timestamp = "2025-01-15T10:35:00" }, wantProblems: []string{"with a timezone"}},
		{name: "not a date", mutate: func(r *updateLocationRequest) { r.Timestamp = "yesterday" }, wantProblems: []string{"must be ISO 8601"}},
		{name: "slightly ahead is allowed", mutate: func(r *updateLocationRequest) { r.Timestamp = "2025-01-15T12:04:00Z" }},
		{name: "far future", mutate: func(r *updateLocationRequest) { r.Timestamp = "2025-01-15T12:06:00Z" }, wantProblems: []string{"in the future"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := valid
			tt.mutate(&req)
			update, problems := req.validate(now)
			assertProblems(t, problems, tt.wantProblems)
			if tt.wantProblems == nil && update.RecordedAt.IsZero() {
				t.Error("RecordedAt not set")
			}
		})
	}
}

func TestParseHistoryFilter(t *testing.T) {
	tests := []struct {
		name         string
		query        string
		wantLimit    int
		wantOffset   int
		wantProblems []string
	}{
		{name: "defaults", query: "", wantLimit: 50},
		{name: "limit and offset", query: "limit=100&offset=200", wantLimit: 100, wantOffset: 200},
		{name: "limit above max is clamped", query: "limit=10000", wantLimit: 500},
		{name: "limit zero", query: "limit=0", wantProblems: []string{"must be a positive integer"}},
		{name: "limit text", query: "limit=ten", wantProblems: []string{"must be a positive integer"}},
		{name: "negative offset", query: "offset=-1", wantProblems: []string{"non-negative integer"}},
		{name: "valid dates", query: "start_date=2025-01-01T00:00:00Z&end_date=2025-02-01T00:00:00Z", wantLimit: 50},
		{name: "bad date", query: "start_date=2025-01-01", wantProblems: []string{"start_date"}},
		{name: "reversed dates", query: "start_date=2025-02-01T00:00:00Z&end_date=2025-01-01T00:00:00Z", wantProblems: []string{"before or equal"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, _ := url.ParseQuery(tt.query)
			filter, problems := parseHistoryFilter(query)
			assertProblems(t, problems, tt.wantProblems)
			if tt.wantProblems == nil && (filter.Limit != tt.wantLimit || filter.Offset != tt.wantOffset) {
				t.Errorf("limit/offset = %d/%d, want %d/%d", filter.Limit, filter.Offset, tt.wantLimit, tt.wantOffset)
			}
		})
	}
}

func TestParseHistoryFilterKeepsIDs(t *testing.T) {
	query, _ := url.ParseQuery("vehicle_id=veh_1&geofence_id=geo_2")
	filter, _ := parseHistoryFilter(query)
	if filter.VehicleID != "veh_1" || filter.GeofenceID != "geo_2" {
		t.Errorf("filter = %+v", filter)
	}
}
