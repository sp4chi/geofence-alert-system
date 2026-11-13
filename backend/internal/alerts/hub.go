package alerts

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait       = 10 * time.Second // max time for one write to a client
	pongWait        = 60 * time.Second // client must answer a ping within this, or it's considered dead
	pingPeriod      = 50 * time.Second // must be < pongWait so the pong can arrive in time
	clientBuffer    = 32               // messages queued per client before it counts as "slow"
	broadcastBuffer = 256              // alerts queued for the hub before Publish starts skipping live delivery
)

// Hub fans alerts out to every connected WebSocket client.
//
// One goroutine (Run) owns the client set and is the only code that touches it; everything else
// talks to it through channels. That's why the map needs no mutex.
type Hub struct {
	register   chan *client
	unregister chan *client
	broadcast  chan []byte
	done       chan struct{} // closed when Run returns, so nobody blocks on a stopped hub
	clients    map[*client]struct{}
}

func NewHub() *Hub {
	return &Hub{
		register:   make(chan *client),
		unregister: make(chan *client),
		broadcast:  make(chan []byte, broadcastBuffer),
		done:       make(chan struct{}),
		clients:    make(map[*client]struct{}),
	}
}

// Run processes registrations and broadcasts until ctx is cancelled, then disconnects everyone.
func (h *Hub) Run(ctx context.Context) {
	defer close(h.done)
	for {
		select {
		case <-ctx.Done():
			for c := range h.clients {
				h.remove(c)
			}
			return

		case c := <-h.register:
			h.clients[c] = struct{}{}
			slog.Info("ws client connected", "clients", len(h.clients))

		case c := <-h.unregister:
			if _, ok := h.clients[c]; ok {
				h.remove(c)
				slog.Info("ws client disconnected", "clients", len(h.clients))
			}

		case msg := <-h.broadcast:
			for c := range h.clients {
				select {
				case c.send <- msg:
				default:
					// A full buffer means the client can't keep up. Dropping it protects everyone
					// else and our memory; it reconnects and backfills from /violations/history.
					h.remove(c)
					slog.Warn("dropped slow ws client", "clients", len(h.clients))
				}
			}
		}
	}
}

// remove deletes the client and closes its send channel, which tells its write pump to hang up.
// Only Run calls this, so each channel is closed exactly once.
func (h *Hub) remove(c *client) {
	delete(h.clients, c)
	close(c.send)
}

// Publish queues an alert for delivery and never blocks the caller (the HTTP request path).
// If the queue is full the live push is skipped; the alert is already stored in the database.
func (h *Hub) Publish(msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		slog.Error("encode alert", "event_id", msg.EventID, "err", err)
		return
	}
	select {
	case h.broadcast <- data:
	default:
		slog.Warn("alert queue full, live delivery skipped (alert is stored)", "event_id", msg.EventID)
	}
}

// Attach takes ownership of an upgraded connection and starts its read and write pumps.
func (h *Hub) Attach(conn *websocket.Conn) {
	c := &client{hub: h, conn: conn, send: make(chan []byte, clientBuffer)}
	select {
	case h.register <- c:
	case <-h.done:
		_ = conn.Close()
		return
	}
	go c.writePump()
	go c.readPump()
}

func (h *Hub) leave(c *client) {
	select {
	case h.unregister <- c:
	case <-h.done: // hub already stopped and closed every client
	}
}

// client is one browser tab. gorilla/websocket allows one concurrent reader and one concurrent
// writer per connection, so readPump only reads and writePump only writes.
type client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

// readPump exists to notice disconnects and handle pong replies; clients never send us data.
func (c *client) readPump() {
	defer func() {
		c.hub.leave(c)
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(512)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait)) // alive: extend the deadline
	})
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return // closed by the client, network drop, or missed pong
		}
	}
}

// writePump sends queued alerts and a ping every pingPeriod to detect dead connections.
func (c *client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok { // hub removed us
				_ = c.conn.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseGoingAway, "server closed connection"))
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
