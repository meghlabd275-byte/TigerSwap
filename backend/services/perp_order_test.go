package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// callStringUserHandler drives a handler whose identity comes from a UUID
// string, matching what the JWT middleware places in the context.
func callStringUserHandler(t *testing.T, handler gin.HandlerFunc, userID, method, target, body string, params gin.Params) (int, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = params
	c.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", userID)
	handler(c)
	return recorder.Code, recorder.Body.String()
}

// TestCreateOrderValidatesOrderShape verifies a malformed order is refused
// before it can reach the matching path.
func TestCreateOrderValidatesOrderShape(t *testing.T) {
	engine := newMarkedPerpEngine(map[string]float64{"BTC": 100})
	ctx := context.Background()

	cases := []struct {
		name                  string
		side, orderType, size string
		price, trigger        float64
		expires               int
	}{
		{"bad side", "up", "market", "1", 0, 0, 60},
		{"bad type", "long", "iceberg", "1", 0, 0, 60},
		{"limit without price", "long", "limit", "1", 0, 0, 60},
		{"stop without trigger", "long", "stop", "1", 0, 0, 60},
		{"empty size", "long", "market", "", 0, 0, 60},
		{"non positive expiry", "long", "market", "1", 0, 0, 0},
	}
	for _, tc := range cases {
		if _, err := engine.CreateOrder(ctx, "user-1", "BTC/RUSD", tc.side, tc.orderType, tc.size, tc.price, tc.trigger, tc.expires); err == nil {
			t.Errorf("%s: order must be rejected", tc.name)
		}
	}
}

// TestCreateOrderHandlerIgnoresBodyIdentity verifies the order is owned by the
// authenticated caller and a user_address in the body cannot override it.
func TestCreateOrderHandlerIgnoresBodyIdentity(t *testing.T) {
	engine := newMarkedPerpEngine(map[string]float64{"BTC": 100})

	body := `{"user_address":"attacker","pair":"BTC/RUSD","side":"long","order_type":"limit","size":"1","price":90,"expires_in":60}`
	status, resp := callStringUserHandler(t, engine.CreateOrderHandler, "7", http.MethodPost, "/perpetual/orders", body, nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", status, resp)
	}

	var order PerpetualOrder
	if err := json.Unmarshal([]byte(resp), &order); err != nil {
		t.Fatalf("decode order: %v", err)
	}
	if order.UserAddress != "7" {
		t.Fatalf("order must belong to the authenticated caller, got %q", order.UserAddress)
	}
}

// TestCancelOrderHandlerEnforcesOwnership verifies one trader cannot cancel
// another trader's order.
func TestCancelOrderHandlerEnforcesOwnership(t *testing.T) {
	engine := newMarkedPerpEngine(map[string]float64{"BTC": 100})

	order, err := engine.CreateOrder(context.Background(), "owner-1", "BTC/RUSD", "long", "limit", "1", 90, 0, 60)
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	status, _ := callStringUserHandler(t, engine.CancelOrderHandler, "2", http.MethodDelete,
		"/perpetual/orders/x", "", gin.Params{{Key: "id", Value: order.ID}})
	if status != http.StatusBadRequest {
		t.Fatalf("a non-owner must not cancel the order, got %d", status)
	}
	if engine.orders[order.ID].Status != "pending" {
		t.Fatalf("a refused cancel must leave the order pending, got %q", engine.orders[order.ID].Status)
	}
}

// TestCancelOrderOwnerSucceeds verifies the owning caller can cancel.
func TestCancelOrderOwnerSucceeds(t *testing.T) {
	engine := newMarkedPerpEngine(map[string]float64{"BTC": 100})

	order, err := engine.CreateOrder(context.Background(), "7", "BTC/RUSD", "long", "limit", "1", 90, 0, 60)
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	status, body := callStringUserHandler(t, engine.CancelOrderHandler, "7", http.MethodDelete,
		"/perpetual/orders/x", "", gin.Params{{Key: "id", Value: order.ID}})
	if status != http.StatusOK {
		t.Fatalf("owner should cancel, got %d: %s", status, body)
	}
	if engine.orders[order.ID].Status != "cancelled" {
		t.Fatalf("order should be cancelled, got %q", engine.orders[order.ID].Status)
	}
}
