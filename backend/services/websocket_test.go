package services

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// TestWebSocketDoesNotEchoClientFrames pins the feed to server-push only. The
// handler used to call Broadcast on every inbound frame, so any connected
// client could inject an arbitrary tick (for example a fake price) into every
// other client's feed. A client frame must be discarded, while a server-side
// Broadcast must still be delivered.
func TestWebSocketDoesNotEchoClientFrames(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go wsHub.Run(ctx)

	r := gin.New()
	r.GET("/ws", HandleWebSocket)
	srv := httptest.NewServer(r)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"

	dial := func() *websocket.Conn {
		conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("dial failed: %v (resp=%v)", err, resp)
		}
		return conn
	}

	base := baselineClients()

	victim := dial()
	defer victim.Close()
	waitForClients(t, base+1)

	attacker := dial()
	defer attacker.Close()
	waitForClients(t, base+2)

	// Read frames off the victim in a goroutine. A read that times out corrupts a
	// gorilla connection, so the victim is only ever read to completion here.
	frames := make(chan string, 16)
	go func() {
		for {
			_, msg, err := victim.ReadMessage()
			if err != nil {
				close(frames)
				return
			}
			frames <- string(msg)
		}
	}()

	// The attacker tries to inject a spoofed frame.
	if err := attacker.WriteMessage(websocket.TextMessage, []byte(`{"type":"price","symbol":"BTC/RUSD","price_usd":1}`)); err != nil {
		t.Fatalf("attacker write failed: %v", err)
	}

	// The victim must not receive the attacker's frame.
	select {
	case msg, ok := <-frames:
		if ok {
			t.Fatalf("client frame was echoed to another client: %s", msg)
		}
		t.Fatal("victim connection closed unexpectedly")
	case <-time.After(700 * time.Millisecond):
	}

	// Sanity: a real server-side broadcast still reaches the victim, so the
	// negative result above is not a dead connection.
	Broadcast([]byte(`{"type":"price","symbol":"BTC/RUSD","price_usd":42}`))
	select {
	case msg, ok := <-frames:
		if !ok {
			t.Fatal("victim connection closed before the broadcast arrived")
		}
		if !strings.Contains(msg, "42") {
			t.Fatalf("unexpected broadcast payload: %s", msg)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server broadcast was not delivered")
	}
}

func waitForClients(t *testing.T, want int) {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		wsHub.mu.RLock()
		n := len(wsHub.clients)
		wsHub.mu.RUnlock()
		if n >= want {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("hub registered %d clients, want %d", n, want)
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
}

func baselineClients() int {
	wsHub.mu.RLock()
	defer wsHub.mu.RUnlock()
	return len(wsHub.clients)
}
