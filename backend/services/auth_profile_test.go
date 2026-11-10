package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"tigerswap/backend/models"
)

// TestUpdateProfileRejectsAnUnsafeUsername pins that a profile update cannot set
// a display name the sign-up path would have refused. The name is rendered back
// to other users, so markup or control characters must not reach storage through
// the update route.
func TestUpdateProfileRejectsAnUnsafeUsername(t *testing.T) {
	db := newListingTestDB(t)
	if err := db.AutoMigrate(&models.User{}); err != nil {
		t.Fatalf("migrate users: %v", err)
	}
	t.Setenv("JWT_SECRET", "test-secret-for-profile-suite-0123456789")
	svc := NewAuthService(db)

	user := models.User{
		ID:       uuid.New(),
		Email:    "trader@example.com",
		Username: "trader",
		Role:     "user",
		IsActive: true,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	update := func(body string) *httptest.ResponseRecorder {
		gin.SetMode(gin.TestMode)
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/auth/profile", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set("user_id", user.ID.String())
		svc.UpdateProfile(c)
		return rec
	}

	bad := `{"username":"<script>alert(1)</script>"}`
	if rec := update(bad); rec.Code != http.StatusBadRequest {
		t.Fatalf("markup username must be rejected, got %d %s", rec.Code, rec.Body.String())
	}

	var stored models.User
	if err := db.Where("id = ?", user.ID).First(&stored).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if stored.Username != "trader" {
		t.Fatalf("rejected update must not change the row, got %q", stored.Username)
	}

	good := `{"username":"trader_2"}`
	rec := update(good)
	if rec.Code != http.StatusOK {
		t.Fatalf("valid username must be accepted, got %d %s", rec.Code, rec.Body.String())
	}
	if err := db.Where("id = ?", user.ID).First(&stored).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if stored.Username != "trader_2" {
		t.Fatalf("valid update must persist, got %q", stored.Username)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
}
