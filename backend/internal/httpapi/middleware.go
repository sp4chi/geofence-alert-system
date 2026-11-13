package httpapi

import (
	"bufio"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"time"
)

type middleware func(http.Handler) http.Handler

// chain wraps h so the first middleware listed is the outermost (runs first).
func chain(h http.Handler, middlewares ...middleware) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

// recoverPanics turns a panic in a handler into a JSON 500 instead of a dropped connection.
func recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			if rec == http.ErrAbortHandler { // deliberate abort; let net/http handle it
				panic(rec)
			}
			slog.Error("panic in handler", "path", r.URL.Path, "panic", rec, "stack", string(debug.Stack()))
			respondError(w, http.StatusInternalServerError, start, "internal server error")
		}()
		next.ServeHTTP(w, r)
	})
}

// logRequests writes one structured log line per request.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.statusOrOK(),
			"duration_ms", float64(time.Since(start).Microseconds())/1000,
		)
	})
}

// originPolicy is the CORS_ORIGINS allow-list, shared by CORS and the WebSocket handshake.
type originPolicy struct {
	allowAll bool
	allowed  map[string]bool
}

func newOriginPolicy(origins []string) originPolicy {
	p := originPolicy{allowed: make(map[string]bool, len(origins))}
	for _, origin := range origins {
		p.allowAll = p.allowAll || origin == "*"
		p.allowed[origin] = true
	}
	return p
}

// allows reports whether a request from origin may proceed. No Origin header means a non-browser
// client (curl, websocat, the evaluator's tests), which CORS is not meant to restrict.
func (p originPolicy) allows(origin string) bool {
	return origin == "" || p.allowAll || p.allowed[origin]
}

// cors allows browsers on the listed origins to call the API. "*" in the list allows any origin.
// We echo the request's Origin rather than sending "*" so the allow-list is real.
func cors(policy originPolicy) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && policy.allows(origin) {
				h := w.Header()
				h.Set("Access-Control-Allow-Origin", origin)
				h.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				h.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
				h.Set("Access-Control-Max-Age", "600")
			}
			w.Header().Add("Vary", "Origin") // caches must not reuse one origin's answer for another

			// Preflight: the browser asks before the real request. Answer with no body.
			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// statusRecorder remembers the status code for logging.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	if s.status == 0 {
		s.status = code
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	return s.ResponseWriter.Write(b)
}

func (s *statusRecorder) statusOrOK() int {
	if s.status == 0 {
		return http.StatusOK
	}
	return s.status
}

// Hijack lets the WebSocket upgrade take over the connection through this wrapper.
func (s *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := s.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("underlying ResponseWriter does not support hijacking")
	}
	s.status = http.StatusSwitchingProtocols
	return hijacker.Hijack()
}

// Unwrap lets http.ResponseController reach the real writer (flush, deadlines).
func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }
