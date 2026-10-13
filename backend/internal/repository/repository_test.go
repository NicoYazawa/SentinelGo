package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"

	"sentinelgo/internal/model"
)

func TestUserRepositoryCRUD(t *testing.T) {
	db := openTestDB(t)
	repo := NewUserRepository(db)

	u := &model.User{Username: "alice", Password: "hash", Status: 1, Nickname: "Alice"}
	if err := repo.Create(u); err != nil {
		t.Fatal(err)
	}
	if u.ID == 0 {
		t.Fatal("id not set")
	}

	got, err := repo.FindByID(u.ID)
	if err != nil || got.Username != "alice" {
		t.Fatalf("FindByID: %v %+v", err, got)
	}

	got.Nickname = "Alice2"
	if err := repo.Update(got); err != nil {
		t.Fatal(err)
	}
	again, _ := repo.FindByID(u.ID)
	if again.Nickname != "Alice2" {
		t.Errorf("update failed: %q", again.Nickname)
	}

	list, err := repo.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("List: %v len=%d", err, len(list))
	}
}

func TestUserRepositoryAccountFlow(t *testing.T) {
	db := openTestDB(t)
	mustCreateRole(t, db, "user", 1)
	repo := NewUserRepository(db)

	user := &model.User{Username: "bob", Password: "ph", Status: 1}
	identity := &model.UserAuth{IdentityType: "password", Identifier: "bob", Credential: "ph"}
	if err := repo.CreateAccount(user, identity); err != nil {
		t.Fatal(err)
	}
	if user.ID == 0 {
		t.Fatal("account id not set")
	}
	auths, err := NewUserAuthRepository(db).ListByUserID(user.ID)
	if err != nil || len(auths) != 1 {
		t.Fatalf("auths: %v %d", err, len(auths))
	}

	found, err := repo.FindByUsername("bob")
	if err != nil || found.ID != user.ID {
		t.Fatalf("FindByUsername: %v %+v", err, found)
	}
	if _, err := repo.FindByUsername("nobody"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("expected not found, got %v", err)
	}

	if err := repo.UpdateStatus(user.ID, 0); err != nil {
		t.Fatal(err)
	}
	found, _ = repo.FindByID(user.ID)
	if found.Status != 0 {
		t.Errorf("status not updated: %d", found.Status)
	}

	if err := repo.UpdatePassword(user.ID, "newhash"); err != nil {
		t.Fatal(err)
	}
	found, _ = repo.FindByID(user.ID)
	if found.Password != "newhash" {
		t.Errorf("password not updated: %q", found.Password)
	}

	if err := repo.UpdateAvatar(user.ID, "http://a.png"); err != nil {
		t.Fatal(err)
	}
	found, _ = repo.FindByID(user.ID)
	if found.Avatar != "http://a.png" {
		t.Errorf("avatar not updated: %q", found.Avatar)
	}

	if err := repo.UpdateDetails(user.ID, map[string]any{"nickname": "Bobby"}); err != nil {
		t.Fatal(err)
	}
	found, _ = repo.FindByID(user.ID)
	if found.Nickname != "Bobby" {
		t.Errorf("details not updated: %q", found.Nickname)
	}

	roles, err := repo.GetEnabledRoles(user.ID)
	if err != nil || len(roles) != 1 {
		t.Fatalf("enabled roles: %v %d", err, len(roles))
	}

	if err := repo.Delete(user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.FindByID(user.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("expected deleted, got %v", err)
	}
}

func TestUserRepositoryAssignRolesAndBootstrap(t *testing.T) {
	db := openTestDB(t)
	r1 := mustCreateRole(t, db, "user", 1)
	r2 := mustCreateRole(t, db, "admin", 1)
	repo := NewUserRepository(db)

	user := &model.User{Username: "carol", Password: "ph", Status: 1}
	if err := db.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.AssignRoles(user.ID, []uint64{r1.ID, r2.ID}); err != nil {
		t.Fatal(err)
	}
	roles, err := repo.GetUserRoles(user.ID)
	if err != nil || len(roles) != 2 {
		t.Fatalf("assigned roles: %v %d", err, len(roles))
	}

	// bootstrap completion requires a valid auth session
	sess := &model.AuthSession{ID: "s1", UserID: user.ID, RefreshJTI: "jti-old", ExpiresAt: time.Now().Add(time.Hour)}
	if err := NewAuthSessionRepository(db).Create(sess); err != nil {
		t.Fatal(err)
	}
	user.PasswordChangeRequired = true
	user.BootstrapCompleted = false
	user.Status = 1
	db.Save(user)
	if err := repo.CompleteBootstrapPassword(user.ID, "s1", "ph", "newph"); err != nil {
		t.Fatal(err)
	}
	found, _ := repo.FindByID(user.ID)
	if !found.BootstrapCompleted || found.PasswordChangeRequired {
		t.Errorf("bootstrap not completed: %+v", found)
	}
	sessRepo := NewAuthSessionRepository(db)
	if err := sessRepo.Revoke("s1", user.ID); err != nil {
		t.Fatal(err)
	}
}

func TestUserRepositoryListWithKeyword(t *testing.T) {
	db := openTestDB(t)
	repo := NewUserRepository(db)
	db.Create(&model.User{Username: "alpha", Nickname: "A", Email: "a@x.com", Status: 1})
	db.Create(&model.User{Username: "beta", Nickname: "B", Status: 1})

	all, total, err := repo.ListWithKeyword("", 1, 10)
	if err != nil || total != 2 || len(all) != 2 {
		t.Fatalf("list all: %v total=%d", err, total)
	}
	hit, total, err := repo.ListWithKeyword("alph", 1, 10)
	if err != nil || total != 1 {
		t.Fatalf("list keyword: %v total=%d", err, total)
	}
	if len(hit) != 1 || hit[0].Username != "alpha" {
		t.Errorf("keyword filter wrong: %+v", hit)
	}
}

func TestUserAuthRepository(t *testing.T) {
	db := openTestDB(t)
	repo := NewUserAuthRepository(db)
	auth := &model.UserAuth{UserID: 1, IdentityType: "phone", Identifier: "13800000000", Credential: "x"}
	if err := repo.Create(auth); err != nil {
		t.Fatal(err)
	}
	got, err := repo.FindByIdentity("phone", "13800000000")
	if err != nil || got.UserID != 1 {
		t.Fatalf("FindByIdentity: %v %+v", err, got)
	}
	if _, err := repo.FindByIdentity("phone", "000"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("expected not found: %v", err)
	}
	list, err := repo.ListByUserID(1)
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v %d", err, len(list))
	}
	if err := repo.DeleteByType(1, "phone"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.FindByIdentity("phone", "13800000000"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("expected deleted: %v", err)
	}
}

func TestRoleRepository(t *testing.T) {
	db := openTestDB(t)
	repo := NewRoleRepository(db)
	r := &model.Role{Name: "editor", Code: "editor", Desc: "d", Status: 1}
	if err := repo.Create(r); err != nil {
		t.Fatal(err)
	}
	got, err := repo.FindByCode("editor")
	if err != nil || got.ID != r.ID {
		t.Fatalf("FindByCode: %v %+v", err, got)
	}
	if _, err := repo.FindByCode("nope"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("expected not found: %v", err)
	}
	enabled, err := repo.ListByStatus(1)
	if err != nil || len(enabled) != 1 {
		t.Fatalf("ListByStatus: %v %d", err, len(enabled))
	}
	if err := repo.UpdateStatus(r.ID, 0); err != nil {
		t.Fatal(err)
	}
	got, _ = repo.FindByID(r.ID)
	if got.Status != 0 {
		t.Errorf("role status not updated")
	}
	// menus
	m := &model.Menu{Name: "m1", Type: 1, Status: 1}
	db.Create(m)
	if err := repo.AssignRoleMenus(r.ID, []uint64{m.ID}); err != nil {
		t.Fatal(err)
	}
	menus, err := repo.GetRoleMenus(r.ID)
	if err != nil || len(menus) != 1 {
		t.Fatalf("GetRoleMenus: %v %d", err, len(menus))
	}
	hit, err := repo.ListWithKeyword("edit")
	if err != nil || len(hit) != 1 {
		t.Fatalf("ListWithKeyword: %v %d", err, len(hit))
	}
	if err := repo.DeleteCascade(r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.FindByID(r.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("expected deleted: %v", err)
	}
}

func TestPermissionRepository(t *testing.T) {
	db := openTestDB(t)
	repo := NewPermissionRepository(db)
	p := &model.Permission{Path: "/api/x", Method: "GET", Description: "d", Status: 1}
	if err := repo.Create(p); err != nil {
		t.Fatal(err)
	}
	got, err := repo.FindByPathAndMethod("/api/x", "GET")
	if err != nil || got.ID != p.ID {
		t.Fatalf("FindByPathAndMethod: %v %+v", err, got)
	}
	if _, err := repo.FindByPathAndMethod("/nope", "GET"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("expected not found: %v", err)
	}
	enabled, err := repo.FindByStatus(1)
	if err != nil || len(enabled) != 1 {
		t.Fatalf("FindByStatus: %v %d", err, len(enabled))
	}
	list, total, err := repo.ListWithFilter("", "", nil, 1, 10)
	if err != nil || total != 1 || len(list) != 1 {
		t.Fatalf("ListWithFilter: %v total=%d", err, total)
	}
	status1 := int8(1)
	list, total, err = repo.ListWithFilter("x", "GET", &status1, 1, 10)
	if err != nil || total != 1 {
		t.Fatalf("ListWithFilter filtered: %v total=%d", err, total)
	}
	if err := repo.Delete(p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.FindByID(p.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("expected deleted: %v", err)
	}
}

func TestMenuRepository(t *testing.T) {
	db := openTestDB(t)
	repo := NewMenuRepository(db)
	m1 := &model.Menu{Name: "root", Type: 1, Sort: 2, Status: 1, Path: "/r"}
	m2 := &model.Menu{Name: "child", Type: 2, Sort: 1, Status: 1, Path: "/c", ParentID: m1.ID}
	if err := repo.Create(m1); err != nil {
		t.Fatal(err)
	}
	m2.ParentID = m1.ID
	if err := repo.Create(m2); err != nil {
		t.Fatal(err)
	}
	all, err := repo.ListTree()
	if err != nil || len(all) != 2 {
		t.Fatalf("ListTree: %v %d", err, len(all))
	}
	byType, err := repo.ListByType(1)
	if err != nil || len(byType) != 1 {
		t.Fatalf("ListByType: %v %d", err, len(byType))
	}
	got, err := repo.FindByPath("/c")
	if err != nil || got.ID != m2.ID {
		t.Fatalf("FindByPath: %v %+v", err, got)
	}
	// user menus require role linkage
	role := mustCreateRole(t, db, "user", 1)
	db.Create(&model.UserRole{UserID: 1, RoleID: role.ID})
	db.Create(&model.RoleMenu{RoleID: role.ID, MenuID: m1.ID})
	menus, err := repo.GetUserMenus(1)
	if err != nil || len(menus) != 1 {
		t.Fatalf("GetUserMenus: %v %d", err, len(menus))
	}
}

func TestDashboardRepository(t *testing.T) {
	db := openTestDB(t)
	repo := NewDashboardRepository(db)
	db.Create(&model.User{Username: "u1", Status: 1})
	db.Create(&model.OperationLog{UserID: 1, Module: "x", Method: "GET", Path: "/a", Status: 1})

	n, err := repo.CountAll("sys_user")
	if err != nil || n != 1 {
		t.Fatalf("CountAll: %v %d", err, n)
	}
	n, err = repo.CountToday("sys_operation_log", "", nil)
	if err != nil || n != 1 {
		t.Fatalf("CountToday: %v %d", err, n)
	}
	n, err = repo.CountToday("sys_operation_log", "module = ?", "x")
	if err != nil || n != 1 {
		t.Fatalf("CountToday filtered: %v %d", err, n)
	}
	ops, err := repo.RecentOperations(10, false, false)
	if err != nil || len(ops) != 1 {
		t.Fatalf("RecentOperations: %v %d", err, len(ops))
	}
	failed, err := repo.RecentOperations(10, true, false)
	if err != nil || len(failed) != 0 {
		t.Fatalf("RecentOperations failed only: %v %d", err, len(failed))
	}
}

func TestOperationLogRepository(t *testing.T) {
	db := openTestDB(t)
	repo := NewOperationLogRepository(db)
	if err := repo.Create(context.Background(), &model.OperationLog{UserID: 1, Username: "u", Module: "m", Method: "GET", Path: "/p", Status: 1}); err != nil {
		t.Fatal(err)
	}
	list, total, err := repo.List(context.Background(), OpLogQuery{Page: 1, PageSize: 10})
	if err != nil || total != 1 || len(list) != 1 {
		t.Fatalf("List: %v total=%d", err, total)
	}
	list, total, err = repo.List(context.Background(), OpLogQuery{Username: "u", Status: 1, Page: 1, PageSize: 10})
	if err != nil || total != 1 {
		t.Fatalf("List filtered: %v total=%d", err, total)
	}
}

func TestAuthSessionRepository(t *testing.T) {
	db := openTestDB(t)
	repo := NewAuthSessionRepository(db)
	if err := repo.Create(&model.AuthSession{ID: "s1", UserID: 1, RefreshJTI: "jti1", ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Validate("s1", 1); err != nil {
		t.Errorf("valid session should validate: %v", err)
	}
	if err := repo.Validate("", 1); err == nil {
		t.Error("empty id should be invalid")
	}
	if err := repo.Validate("s1", 999); err == nil {
		t.Error("wrong user should be invalid")
	}
	if err := repo.Rotate("s1", 1, "jti1", "jti2"); err != nil {
		t.Errorf("rotate should succeed: %v", err)
	}
	if err := repo.Rotate("s1", 1, "jti1", "jti3"); err == nil {
		t.Error("rotate with stale jti should fail")
	}
	if err := repo.Revoke("s1", 1); err != nil {
		t.Fatal(err)
	}
	if err := repo.Validate("s1", 1); err == nil {
		t.Error("revoked session should be invalid")
	}
}
