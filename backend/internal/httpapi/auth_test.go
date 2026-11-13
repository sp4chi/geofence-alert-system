package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireAPIKey(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	tests := []struct {
		name       string
		configured string
		method     string
		path       string
		headers    map[string]string
		want       int
	}{
		{"off by default: no key needed", "", "GET", "/vehicles", nil, 200},
		{"missing key", "s3cret", "GET", "/vehicles", nil, 401},
		{"wrong key", "s3cret", "GET", "/vehicles", map[string]string{"X-API-Key": "guess"}, 401},
		{"X-API-Key header", "s3cret", "POST", "/vehicles/location", map[string]string{"X-API-Key": "s3cret"}, 200},
		{"bearer token", "s3cret", "GET", "/geofences", map[string]string{"Authorization": "Bearer s3cret"}, 200},
		{"health check stays open", "s3cret", "GET", "/healthz", nil, 200},
		{"CORS preflight stays open", "s3cret", "OPTIONS", "/geofences", nil, 200},
		{"websocket query param", "s3cret", "GET", "/ws/alerts?api_key=s3cret", nil, 200},
		{"query param only works for the websocket", "s3cret", "GET", "/vehicles?api_key=s3cret", nil, 401},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			requireAPIKey(tt.configured)(ok).ServeHTTP(rec, req)

			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d", rec.Code, tt.want)
			}
			if tt.want == http.StatusUnauthorized {
				if body := decode(t, rec); body["error"] == "" {
					t.Error("401 needs an error message")
				}
			}
		})
	}
}
