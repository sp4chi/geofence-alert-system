package httpapi

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

// errorResponse is the body for every non-2xx reply: {"error": "...", "details": [...], "time_ns": "..."}.
type errorResponse struct {
	Error   string   `json:"error"`
	Details []string `json:"details,omitempty"`
}

// respond writes body as JSON with "time_ns" (nanoseconds since start, as a string) appended.
// Every handler must reply through respond or respondError; that is what guarantees time_ns
// is present on success and error paths alike. body must encode to a JSON object.
func respond(w http.ResponseWriter, status int, start time.Time, body any) {
	encoded, err := json.Marshal(body)
	if err != nil || !isJSONObject(encoded) {
		slog.Error("respond: body must encode to a JSON object", "err", err, "type", fmt.Sprintf("%T", body))
		status = http.StatusInternalServerError
		encoded, _ = json.Marshal(errorResponse{Error: "internal server error"})
	}

	// Measured after encoding so serialisation cost is included, just before the write.
	out := appendTimeNS(encoded, time.Since(start))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(out); err != nil {
		slog.Debug("respond: client went away", "err", err)
	}
}

// respondError writes {"error": message, "details": [...], "time_ns": "..."}.
func respondError(w http.ResponseWriter, status int, start time.Time, message string, details ...string) {
	respond(w, status, start, errorResponse{Error: message, Details: details})
}

// appendTimeNS adds "time_ns" as the last field of an encoded JSON object.
// Splicing bytes (instead of a TimeNS field on every struct) means no response type can forget it,
// and keeps time_ns last, as in the README examples.
func appendTimeNS(object []byte, elapsed time.Duration) []byte {
	field := `"time_ns":"` + strconv.FormatInt(elapsed.Nanoseconds(), 10) + `"`
	inner := object[1 : len(object)-1] // between the outer { and }

	out := make([]byte, 0, len(object)+len(field)+1)
	out = append(out, '{')
	out = append(out, inner...)
	if len(inner) > 0 {
		out = append(out, ',')
	}
	out = append(out, field...)
	return append(out, '}')
}

// formatTime renders timestamps as UTC RFC 3339 with whole seconds, e.g. "2025-01-15T10:30:00Z".
func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// isJSONObject is enough for json.Marshal output, which never has surrounding whitespace.
func isJSONObject(encoded []byte) bool {
	return len(encoded) >= 2 && encoded[0] == '{' && encoded[len(encoded)-1] == '}'
}
