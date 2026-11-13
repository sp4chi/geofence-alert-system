package httpapi

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// handleAlertsSocket upgrades GET /ws/alerts to a WebSocket and hands the connection to the hub.
// The HTTP layer only does the handshake; the hub owns the connection afterwards.
func (a *API) handleAlertsSocket(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	upgrader := websocket.Upgrader{
		// Browsers always send Origin on WebSocket handshakes and CORS does not apply to them,
		// so this check is what stops another website from opening our alert stream.
		CheckOrigin: func(r *http.Request) bool {
			return a.origins.allows(r.Header.Get("Origin"))
		},
		// Failed handshakes still answer with our JSON error shape and time_ns.
		Error: func(w http.ResponseWriter, r *http.Request, status int, reason error) {
			respondError(w, status, start, "websocket upgrade failed: "+reason.Error())
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return // Error above has already written the response
	}
	a.hub.Attach(conn)
}
