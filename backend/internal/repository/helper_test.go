package repository

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"sentinelgo/internal/model"
)

// openTestDB opens an isolated in-memory SQLite database and migrates all models.
func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1) // keep a single connection so in-memory data persists

	if err := db.AutoMigrate(
		&model.User{}, &model.Role{}, &model.Menu{}, &model.Permission{},
		&model.CasbinRule{}, &model.OperationLog{}, &model.UserRole{},
		&model.RoleMenu{}, &model.UserAuth{}, &model.AuthSession{}, &model.OAuthState{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

func mustCreateRole(t *testing.T, db *gorm.DB, code string, status int8) *model.Role {
	t.Helper()
	role := &model.Role{Name: code, Code: code, Status: status}
	if err := db.Create(role).Error; err != nil {
		t.Fatal(err)
	}
	return role
}
