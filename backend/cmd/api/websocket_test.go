package main

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"tigerswap/backend/services"
)

// TestWebSocketRouteUpgradesAndReceivesBroadcast exercises the real `/ws`
// route end to end: the hub must be running for the handshake to complete,
// because the handler registers the connection on the hub before it returns.
// Before the hub was started from main, registration blocked and this
// handshake never completed, so the endpoint was unusable.
func TestWebSocketRouteUpgradesAndReceivesBroadcast(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	services.StartWebSocketHub(ctx)

	router := setupRouter(apiDeps{})
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	conn, response, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("websocket handshake failed: %v (status %v)", err, response)
	}
	defer conn.Close()

	// The hub may not have processed the registration yet when the first
	// broadcast is queued, so keep publishing the same tick until the client
	// receives one. A single read with one deadline is used because a gorilla
	// websocket cannot be read again after a read timeout.
	payload := []byte(`{"type":"price","symbol":"BTC","priceUSD":61234.5,"change24h":1.25,"timestamp":1735689600}`)
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		ticker := time.NewTicker(25 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				services.Broadcast(payload)
			}
		}
	}()

	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	_, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("did not receive the broadcast: %v", err)
	}
	if string(message) != string(payload) {
		t.Fatalf("unexpected message: %s", message)
	}
}
