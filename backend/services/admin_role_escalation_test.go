package services

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"tigerswap/backend/models"
)

// adminContextWithRole builds a test context carrying an authenticated actor
// with the given role, the way the auth middleware sets it.
func adminContextWithRole(rec *httptest.ResponseRecorder, method, path, body, role string) *gin.Context {
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	// The handler reads the target from the `id` path parameter; the router is not
	// in the loop here, so it is populated from the trailing path segment.
	if idx := strings.LastIndex(path, "/"); idx >= 0 && idx < len(path)-1 {
		c.Params = gin.Params{{Key: "id", Value: path[idx+1:]}}
	}
	c.Set("user_id", "11111111-1111-1111-1111-111111111111")
	c.Set("role", role)
	return c
}

// newAdminUserTestDB migrates the tables UpdateUser touches.
func newAdminUserTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := newAdminWriteTestDB(t)
	if err := db.AutoMigrate(&models.User{}, &models.AdminLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// seedRoleUser creates an account with the supplied role and returns it.
func seedRoleUser(t *testing.T, svc *AdminService, role string) models.User {
	t.Helper()
	user := models.User{Email: role + "@example.com", Username: role + "-acct", Role: role, IsActive: true}
	if err := svc.db.Create(&user).Error; err != nil {
		t.Fatalf("seed %s: %v", role, err)
	}
	return user
}

func reloadRole(t *testing.T, db *gorm.DB, id interface{}) string {
	t.Helper()
	var reloaded models.User
	if err := db.Where("id = ?", id).First(&reloaded).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	return reloaded.Role
}

// TestUpdateUserBlocksAdminPrivilegeEscalation proves an ordinary `admin` cannot
// grant the superadmin role. Before the guard an admin could promote any account
// to superadmin through PUT /admin/users/:id, which is the highest privilege on
// the platform.
func TestUpdateUserBlocksAdminPrivilegeEscalation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := NewAdminService(newAdminUserTestDB(t))

	victim := seedRoleUser(t, svc, models.RoleUser)

	rec := httptest.NewRecorder()
	c := adminContextWithRole(rec, http.MethodPut, "/admin/users/"+victim.ID.String(),
		`{"role":"superadmin"}`, models.RoleAdmin)
	svc.UpdateUser(c)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("admin promoting to superadmin answered %d, want 403", rec.Code)
	}
	if role := reloadRole(t, svc.db, victim.ID); role != models.RoleUser {
		t.Fatalf("role changed to %q despite the guard", role)
	}
}

// TestUpdateUserBlocksAdminSelfPromotion covers the same escalation aimed at the
// caller's own account, which is the shortest path to a superadmin token.
func TestUpdateUserBlocksAdminSelfPromotion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := NewAdminService(newAdminUserTestDB(t))

	admin := seedRoleUser(t, svc, models.RoleAdmin)

	rec := httptest.NewRecorder()
	c := adminContextWithRole(rec, http.MethodPut, "/admin/users/"+admin.ID.String(),
		`{"role":"superadmin"}`, models.RoleAdmin)
	c.Set("user_id", admin.ID.String())
	svc.UpdateUser(c)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("self-promotion answered %d, want 403", rec.Code)
	}
	if role := reloadRole(t, svc.db, admin.ID); role != models.RoleAdmin {
		t.Fatalf("role changed to %q despite the guard", role)
	}
}

// TestUpdateUserSuperAdminCanGrantSuperAdmin proves the guard is not a blanket
// denial: a superadmin is exactly the actor who may grant the role.
func TestUpdateUserSuperAdminCanGrantSuperAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := NewAdminService(newAdminUserTestDB(t))

	victim := seedRoleUser(t, svc, models.RoleUser)

	rec := httptest.NewRecorder()
	c := adminContextWithRole(rec, http.MethodPut, "/admin/users/"+victim.ID.String(),
		`{"role":"superadmin"}`, models.RoleSuperAdmin)
	svc.UpdateUser(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("superadmin grant answered %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	if role := reloadRole(t, svc.db, victim.ID); role != models.RoleSuperAdmin {
		t.Fatalf("role = %q, want superadmin", role)
	}
}

// TestUpdateUserBlocksAdminDemotingSuperAdmin proves an ordinary admin cannot
// remove a superadmin either: revoking the top role is the same privilege
// transition as granting it.
func TestUpdateUserBlocksAdminDemotingSuperAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := NewAdminService(newAdminUserTestDB(t))

	owner := seedRoleUser(t, svc, models.RoleSuperAdmin)

	rec := httptest.NewRecorder()
	c := adminContextWithRole(rec, http.MethodPut, "/admin/users/"+owner.ID.String(),
		`{"role":"user"}`, models.RoleAdmin)
	svc.UpdateUser(c)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("admin demoting a superadmin answered %d, want 403", rec.Code)
	}
	if role := reloadRole(t, svc.db, owner.ID); role != models.RoleSuperAdmin {
		t.Fatalf("superadmin was demoted to %q despite the guard", role)
	}
}

// TestUpdateUserAdminCanStillManageOrdinaryAccounts proves the guard does not
// break the ordinary admin workflow: promoting a user to admin remains allowed.
func TestUpdateUserAdminCanStillManageOrdinaryAccounts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := NewAdminService(newAdminUserTestDB(t))

	user := seedRoleUser(t, svc, models.RoleUser)

	rec := httptest.NewRecorder()
	c := adminContextWithRole(rec, http.MethodPut, "/admin/users/"+user.ID.String(),
		`{"role":"admin"}`, models.RoleAdmin)
	svc.UpdateUser(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("admin promoting a user to admin answered %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	if role := reloadRole(t, svc.db, user.ID); role != models.RoleAdmin {
		t.Fatalf("role = %q, want admin", role)
	}
}

// TestDeleteUserBlocksAdminDeactivatingSuperAdmin proves an ordinary admin cannot
// deactivate the platform owner's account, which would otherwise be a denial of
// service against the highest-privilege account.
func TestDeleteUserBlocksAdminDeactivatingSuperAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := NewAdminService(newAdminUserTestDB(t))

	owner := seedRoleUser(t, svc, models.RoleSuperAdmin)

	rec := httptest.NewRecorder()
	c := adminContextWithRole(rec, http.MethodDelete, "/admin/users/"+owner.ID.String(), "", models.RoleAdmin)
	svc.DeleteUser(c)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("admin deactivating a superadmin answered %d, want 403", rec.Code)
	}
}

// TestDeleteUserSuperAdminCanDeactivate proves the guard still lets a superadmin
// deactivate another operator.
func TestDeleteUserSuperAdminCanDeactivate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := NewAdminService(newAdminUserTestDB(t))

	other := seedRoleUser(t, svc, models.RoleAdmin)

	rec := httptest.NewRecorder()
	c := adminContextWithRole(rec, http.MethodDelete, "/admin/users/"+other.ID.String(), "", models.RoleSuperAdmin)
	svc.DeleteUser(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("superadmin deactivation answered %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
}

// TestAuthorizeRoleChange pins the transition matrix directly.
func TestAuthorizeRoleChange(t *testing.T) {
	cases := []struct {
		actor, from, to string
		wantErr         bool
	}{
		{models.RoleSuperAdmin, models.RoleUser, models.RoleSuperAdmin, false},
		{models.RoleSuperAdmin, models.RoleSuperAdmin, models.RoleUser, false},
		{models.RoleSuperAdmin, models.RoleUser, models.RoleAdmin, false},
		{models.RoleAdmin, models.RoleUser, models.RoleSuperAdmin, true},
		{models.RoleAdmin, models.RoleAdmin, models.RoleUser, false},
		{models.RoleAdmin, models.RoleUser, models.RoleAdmin, false},
		{models.RoleAdmin, models.RoleSuperAdmin, models.RoleUser, true},
		{models.RoleAdmin, models.RoleAdmin, models.RoleSuperAdmin, true},
		{models.RoleUser, models.RoleUser, models.RoleAdmin, true},
		{"", models.RoleUser, models.RoleAdmin, true},
	}
	for _, tc := range cases {
		err := authorizeRoleChange(tc.actor, tc.from, tc.to)
		if (err != nil) != tc.wantErr {
			t.Errorf("authorizeRoleChange(%q,%q,%q) err=%v, wantErr=%v", tc.actor, tc.from, tc.to, err, tc.wantErr)
		}
	}
}
