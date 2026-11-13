package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS(t *testing.T) {
	okHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTeapot) })

	tests := []struct {
		name       string
		allowed    []string
		method     string
		origin     string
		preflight  bool
		wantAllow  string
		wantStatus int
	}{
		{"allowed origin", []string{"https://app.vercel.app"}, "GET", "https://app.vercel.app", false, "https://app.vercel.app", http.StatusTeapot},
		{"other origin gets no header", []string{"https://app.vercel.app"}, "GET", "https://evil.com", false, "", http.StatusTeapot},
		{"no origin (curl)", []string{"https://app.vercel.app"}, "GET", "", false, "", http.StatusTeapot},
		{"wildcard", []string{"*"}, "GET", "http://anything", false, "http://anything", http.StatusTeapot},
		{"preflight answered without calling handler", []string{"https://app.vercel.app"}, "OPTIONS", "https://app.vercel.app", true, "https://app.vercel.app", http.StatusNoContent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/geofences", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if tt.preflight {
				req.Header.Set("Access-Control-Request-Method", "POST")
			}
			rec := httptest.NewRecorder()
			cors(newOriginPolicy(tt.allowed))(okHandler).ServeHTTP(rec, req)

			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != tt.wantAllow {
				t.Errorf("Allow-Origin = %q, want %q", got, tt.wantAllow)
			}
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestRecoverPanicsReturnsJSON500(t *testing.T) {
	panicky := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") })
	rec := httptest.NewRecorder()

	chain(panicky, logRequests, recoverPanics).ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d", rec.Code)
	}
	if body := decode(t, rec); body["error"] != "internal server error" {
		t.Errorf("body = %v", body)
	}
}

func TestChainOrder(t *testing.T) {
	var order []string
	mark := func(name string) middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name)
				next.ServeHTTP(w, r)
			})
		}
	}
	final := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { order = append(order, "handler") })

	chain(final, mark("first"), mark("second")).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))

	if got := len(order); got != 3 || order[0] != "first" || order[1] != "second" || order[2] != "handler" {
		t.Errorf("order = %v", order)
	}
}
