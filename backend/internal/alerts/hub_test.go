package alerts

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// startHub runs a hub behind a real WebSocket server. attached receives a value each time a
// connection has been registered, so tests never publish before a client is in the set.
func startHub(t *testing.T) (hub *Hub, url string, attached chan struct{}) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	hub = NewHub()
	go hub.Run(ctx)

	attached = make(chan struct{}, 10)
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		hub.Attach(conn)
		attached <- struct{}{}
	}))
	t.Cleanup(server.Close)
	return hub, "ws" + strings.TrimPrefix(server.URL, "http"), attached
}

func dial(t *testing.T, url string, attached chan struct{}) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	select {
	case <-attached:
	case <-time.After(2 * time.Second):
		t.Fatal("client was not attached")
	}
	return conn
}

func readMessage(t *testing.T, conn *websocket.Conn) Message {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("decode %s: %v", data, err)
	}
	return msg
}

func TestBroadcastReachesEveryClient(t *testing.T) {
	hub, url, attached := startHub(t)
	first := dial(t, url, attached)
	second := dial(t, url, attached)

	hub.Publish(Message{EventID: "evt_1", EventType: "entry"})

	for i, conn := range []*websocket.Conn{first, second} {
		if got := readMessage(t, conn); got.EventID != "evt_1" || got.EventType != "entry" {
			t.Errorf("client %d got %+v", i, got)
		}
	}
}

func TestMessageJSONMatchesREADME(t *testing.T) {
	data, _ := json.Marshal(Message{})
	for _, key := range []string{`"event_id"`, `"event_type"`, `"timestamp"`, `"vehicle":{"vehicle_id"`, `"vehicle_number"`,
		`"driver_name"`, `"geofence":{"geofence_id"`, `"geofence_name"`, `"category"`, `"location":{"latitude"`, `"longitude"`} {
		if !strings.Contains(string(data), key) {
			t.Errorf("payload %s is missing %s", data, key)
		}
	}
}

func TestSlowClientIsDroppedWithoutBlockingOthers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hub := NewHub()
	go hub.Run(ctx)

	// slow never reads: its unbuffered send channel is "full" from the start.
	slow := &client{hub: hub, send: make(chan []byte)}
	healthy := &client{hub: hub, send: make(chan []byte, clientBuffer)}
	hub.register <- slow
	hub.register <- healthy

	hub.Publish(Message{EventID: "evt_1"})
	hub.Publish(Message{EventID: "evt_2"})

	// Receiving both on the healthy client proves it kept getting alerts, and that the hub has
	// finished the first broadcast (where slow was dropped) before we inspect slow.
	for _, want := range []string{"evt_1", "evt_2"} {
		select {
		case data := <-healthy.send:
			if !strings.Contains(string(data), want) {
				t.Fatalf("healthy client got %s, want %s", data, want)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("healthy client did not receive %s", want)
		}
	}

	if _, open := <-slow.send; open {
		t.Fatal("expected the slow client's channel to be closed")
	}
}

func TestPublishNeverBlocks(t *testing.T) {
	hub := NewHub() // Run never started, so nothing drains the queue

	done := make(chan struct{})
	go func() {
		for range broadcastBuffer + 50 {
			hub.Publish(Message{EventID: "evt_x"})
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Publish blocked when the queue was full")
	}
}

func TestShutdownClosesClients(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	hub := NewHub()
	go hub.Run(ctx)

	c := &client{hub: hub, send: make(chan []byte, 1)}
	hub.register <- c
	cancel()

	select {
	case _, open := <-c.send:
		if open {
			t.Fatal("expected closed channel")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("client not closed on shutdown")
	}

	// After shutdown, leaving must not block (nobody reads unregister any more).
	left := make(chan struct{})
	go func() { hub.leave(c); close(left) }()
	select {
	case <-left:
	case <-time.After(2 * time.Second):
		t.Fatal("leave blocked after hub stopped")
	}
}
