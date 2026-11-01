package services

import (
	"context"
	"testing"
	"time"
)

// TestWebSocketHubDeliversBroadcast proves the hub routes a broadcast to a
// registered client. Before StartWebSocketHub was called from main the hub
// goroutine never ran, so registration blocked and no message was ever
// delivered; this test exercises the delivered path directly.
func TestWebSocketHubDeliversBroadcast(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go wsHub.Run(ctx)

	conn := &websocketConn{send: make(chan []byte, 8)}
	wsHub.register <- conn

	// Wait for the hub to accept the registration.
	deadline := time.After(2 * time.Second)
	for {
		wsHub.mu.RLock()
		_, ok := wsHub.clients[conn]
		wsHub.mu.RUnlock()
		if ok {
			break
		}
		select {
		case <-deadline:
			t.Fatal("hub did not register the client")
		default:
			time.Sleep(time.Millisecond)
		}
	}

	Broadcast([]byte("tick"))
	select {
	case msg := <-conn.send:
		if string(msg) != "tick" {
			t.Fatalf("unexpected message %q", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("broadcast was not delivered to the client")
	}

	wsHub.unregister <- conn
}

// TestBroadcastNeverBlocks confirms Broadcast drops rather than blocking when
// the hub queue is full, so an HTTP handler or the market feed can never wedge.
func TestBroadcastNeverBlocks(t *testing.T) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		// Fill well past the channel's capacity.
		for i := 0; i < 1000; i++ {
			Broadcast([]byte("x"))
		}
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Broadcast blocked when the hub queue was full")
	}
}
