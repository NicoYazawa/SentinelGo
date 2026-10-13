package casbin

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestCasbinBasics(t *testing.T) {
	db := newTestDB(t)
	Init(db, "../../config/rbac_model.conf")
	defer func() { Enforcer = nil }()

	if ok, err := AddPolicy("admin", "/api/v1/users", "GET"); err != nil || !ok {
		t.Fatalf("AddPolicy: %v %v", ok, err)
	}
	allowed, err := CheckPermission("admin", "/api/v1/users", "GET")
	if err != nil || !allowed {
		t.Errorf("CheckPermission should allow: %v %v", allowed, err)
	}
	denied, err := CheckPermission("user", "/api/v1/users", "GET")
	if err != nil || denied {
		t.Errorf("CheckPermission should deny: %v %v", denied, err)
	}

	if err := LoadPolicy(); err != nil {
		t.Errorf("LoadPolicy: %v", err)
	}

	if _, err := AddRoleForUser("1", "admin"); err != nil {
		t.Fatal(err)
	}
	roles, err := GetRolesForUser("1")
	if err != nil || len(roles) != 1 {
		t.Errorf("roles wrong: %v %v", roles, err)
	}
	if _, err := RemoveRoleForUser("1", "admin"); err != nil {
		t.Fatal(err)
	}
	if _, err := RemovePolicy("admin", "/api/v1/users", "GET"); err != nil {
		t.Fatal(err)
	}
	if _, err := DeletePermissionsForUser("1"); err != nil {
		t.Fatal(err)
	}
}
