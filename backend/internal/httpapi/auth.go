package httpapi

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"time"
)

// requireAPIKey enforces API_KEY when it is set (it is empty, so off, by default). Clients send
// "X-API-Key: <key>" or "Authorization: Bearer <key>". Browsers cannot set headers on a WebSocket
// handshake, so /ws/alerts also accepts ?api_key=<key>. /healthz stays open for the host's health
// checks, and CORS preflights never carry credentials, so they pass too (cors answers them first).
func requireAPIKey(key string) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if key == "" || r.URL.Path == "/healthz" || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}
			if !keysMatch(presentedKey(r), key) {
				start := time.Now()
				w.Header().Set("WWW-Authenticate", "Bearer")
				respondError(w, http.StatusUnauthorized, start, "missing or invalid API key: send it in the X-API-Key header")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func presentedKey(r *http.Request) string {
	if key := r.Header.Get("X-API-Key"); key != "" {
		return key
	}
	if key, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
		return key
	}
	if r.URL.Path == "/ws/alerts" {
		return r.URL.Query().Get("api_key")
	}
	return ""
}

// keysMatch compares in constant time, so response timing can't reveal how much of a guess was right.
func keysMatch(presented, expected string) bool {
	return subtle.ConstantTimeCompare([]byte(presented), []byte(expected)) == 1
}
