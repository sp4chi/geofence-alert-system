package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"
)

func TestAppendTimeNS(t *testing.T) {
	tests := []struct {
		name    string
		object  string
		elapsed time.Duration
		want    string
	}{
		{"empty object", `{}`, 42, `{"time_ns":"42"}`},
		{"one field", `{"id":"geo_1"}`, 1500, `{"id":"geo_1","time_ns":"1500"}`},
		{"nested", `{"a":{"b":[1,2]}}`, 7, `{"a":{"b":[1,2]},"time_ns":"7"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(appendTimeNS([]byte(tt.object), tt.elapsed))
			if got != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

var timeNSPattern = regexp.MustCompile(`^[0-9]+$`)

// decode checks the shared contract (JSON content type, time_ns as a numeric string) and returns the body.
func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON %q: %v", rec.Body.String(), err)
	}
	timeNS, ok := body["time_ns"].(string)
	if !ok || !timeNSPattern.MatchString(timeNS) {
		t.Errorf("time_ns = %#v, want numeric string", body["time_ns"])
	}
	return body
}

func TestRespond(t *testing.T) {
	rec := httptest.NewRecorder()
	respond(rec, http.StatusCreated, time.Now(), map[string]string{"id": "veh_1"})

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d", rec.Code)
	}
	if body := decode(t, rec); body["id"] != "veh_1" {
		t.Errorf("id = %v", body["id"])
	}
}

func TestRespondRejectsNonObject(t *testing.T) {
	rec := httptest.NewRecorder()
	respond(rec, http.StatusOK, time.Now(), []string{"not", "an", "object"})

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	decode(t, rec)
}

func TestRespondError(t *testing.T) {
	rec := httptest.NewRecorder()
	respondError(rec, http.StatusBadRequest, time.Now(), "invalid polygon", "first and last point differ")

	body := decode(t, rec)
	if rec.Code != http.StatusBadRequest || body["error"] != "invalid polygon" {
		t.Errorf("got %d %v", rec.Code, body)
	}
	if details, _ := body["details"].([]any); len(details) != 1 {
		t.Errorf("details = %v", body["details"])
	}
}

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

func TestRoutes(t *testing.T) {
	tests := []struct {
		name       string
		db         Pinger
		method     string
		path       string
		wantStatus int
	}{
		{"health ok", fakePinger{}, http.MethodGet, "/healthz", http.StatusOK},
		{"health db down", fakePinger{errors.New("down")}, http.MethodGet, "/healthz", http.StatusServiceUnavailable},
		{"unknown route", fakePinger{}, http.MethodGet, "/nope", http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			NewRouter(Options{Health: tt.db}).ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			decode(t, rec)
		})
	}
}
