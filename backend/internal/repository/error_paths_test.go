package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"gorm.io/gorm"

	"sentinelgo/internal/model"
)

func injectRepoFailure(t *testing.T, db *gorm.DB, kind, table string, nth int) (error, func()) {
	t.Helper()
	want := errors.New("repository storage failure")
	calls := 0
	fn := func(tx *gorm.DB) {
		if table == "" || tx.Statement.Table == table {
			calls++
			if calls == nth {
				tx.AddError(want)
			}
		}
	}
	name := "test:repository_failure"
	var err error
	var remove func()
	switch kind {
	case "query":
		err = db.Callback().Query().Before("gorm:query").Register(name, fn)
		remove = func() { db.Callback().Query().Remove(name) }
	case "create":
		err = db.Callback().Create().Before("gorm:create").Register(name, fn)
		remove = func() { db.Callback().Create().Remove(name) }
	case "update":
		err = db.Callback().Update().Before("gorm:update").Register(name, fn)
		remove = func() { db.Callback().Update().Remove(name) }
	case "delete":
		err = db.Callback().Delete().Before("gorm:delete").Register(name, fn)
		remove = func() { db.Callback().Delete().Remove(name) }
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(remove)
	return want, remove
}

func seedRepoAccount(t *testing.T, db *gorm.DB) *model.User {
	t.Helper()
	mustCreateRole(t, db, "user", 1)
	u := &model.User{Username: "rollback", Password: "old", PasswordChangeRequired: true, Status: 1}
	if err := NewUserRepository(db).CreateAccount(u, &model.UserAuth{IdentityType: "password", Identifier: u.Username, Credential: u.Password}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.AuthSession{ID: "rollback-session", UserID: u.ID, RefreshJTI: "jti", ExpiresAt: time.Now().Add(time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	return u
}

func TestAccountMutationsRollbackOnStorageFailure(t *testing.T) {
	cases := []struct {
		name, kind, table string
		nth               int
		run               func(*UserRepository, *model.User) error
	}{
		{"status", "update", "sys_user", 1, func(r *UserRepository, u *model.User) error { return r.UpdateStatus(u.ID, 0) }},
		{"password write", "update", "sys_user", 1, func(r *UserRepository, u *model.User) error { return r.UpdatePassword(u.ID, "new") }},
		{"password reload", "query", "sys_user", 1, func(r *UserRepository, u *model.User) error { return r.UpdatePassword(u.ID, "new") }},
		{"password identity", "update", "user_auths", 1, func(r *UserRepository, u *model.User) error { return r.UpdatePassword(u.ID, "new") }},
		{"bootstrap lookup", "query", "sys_user", 1, func(r *UserRepository, u *model.User) error {
			return r.CompleteBootstrapPassword(u.ID, "rollback-session", "old", "new")
		}},
		{"bootstrap write", "update", "sys_user", 1, func(r *UserRepository, u *model.User) error {
			return r.CompleteBootstrapPassword(u.ID, "rollback-session", "old", "new")
		}},
		{"bootstrap identity", "update", "user_auths", 1, func(r *UserRepository, u *model.User) error {
			return r.CompleteBootstrapPassword(u.ID, "rollback-session", "old", "new")
		}},
		{"details", "update", "sys_user", 1, func(r *UserRepository, u *model.User) error {
			return r.UpdateDetails(u.ID, map[string]any{"username": "new"})
		}},
		{"details identity", "update", "user_auths", 1, func(r *UserRepository, u *model.User) error {
			return r.UpdateDetails(u.ID, map[string]any{"username": "new"})
		}},
		{"delete revoke", "update", "auth_sessions", 1, func(r *UserRepository, u *model.User) error { return r.Delete(u.ID) }},
		{"delete roles", "delete", "user_roles", 1, func(r *UserRepository, u *model.User) error { return r.Delete(u.ID) }},
		{"delete identities", "delete", "user_auths", 1, func(r *UserRepository, u *model.User) error { return r.Delete(u.ID) }},
		{"delete policies", "delete", "casbin_rule", 1, func(r *UserRepository, u *model.User) error { return r.Delete(u.ID) }},
		{"assign user lookup", "query", "sys_user", 1, func(r *UserRepository, u *model.User) error { return r.AssignRoles(u.ID, []uint64{1}) }},
		{"assign role lookup", "query", "sys_role", 1, func(r *UserRepository, u *model.User) error { return r.AssignRoles(u.ID, []uint64{1}) }},
		{"assign association", "create", "sys_role", 1, func(r *UserRepository, u *model.User) error { return r.AssignRoles(u.ID, []uint64{1}) }},
		{"assign policy delete", "delete", "casbin_rule", 1, func(r *UserRepository, u *model.User) error { return r.AssignRoles(u.ID, []uint64{1}) }},
		{"assign policy create", "create", "casbin_rule", 1, func(r *UserRepository, u *model.User) error { return r.AssignRoles(u.ID, []uint64{1}) }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			db := openTestDB(t)
			u := seedRepoAccount(t, db)
			want, remove := injectRepoFailure(t, db, c.kind, c.table, c.nth)
			if err := c.run(NewUserRepository(db), u); !errors.Is(err, want) {
				t.Fatalf("error = %v, want %v", err, want)
			}
			remove()
			got, err := NewUserRepository(db).FindForAuth(u.ID)
			if err != nil || got.Username != u.Username || got.Password != "old" || got.Status != 1 || !got.PasswordChangeRequired {
				t.Fatalf("account mutation escaped rollback: %+v %v", got, err)
			}
			if err := NewAuthSessionRepository(db).Validate("rollback-session", u.ID); err != nil {
				t.Fatalf("session revoked despite rollback: %v", err)
			}
			identities, err := NewUserAuthRepository(db).ListByUserID(u.ID)
			if err != nil || len(identities) != 1 || identities[0].Identifier != u.Username || identities[0].Credential != "old" {
				t.Fatalf("identity mutation escaped rollback: %+v %v", identities, err)
			}
			roles, err := NewUserRepository(db).GetUserRoles(u.ID)
			if err != nil || len(roles) != 1 {
				t.Fatalf("role mutation escaped rollback: %+v %v", roles, err)
			}
			var n int64
			db.Model(&model.CasbinRule{}).Where("ptype = ? AND v0 = ?", "g", fmt.Sprint(u.ID)).Count(&n)
			if n != 1 {
				t.Fatalf("Casbin role mapping removed: %d", n)
			}
		})
	}
}

func TestCreateAccountFailureLeavesNoPartialAccount(t *testing.T) {
	for _, table := range []string{"sys_role", "sys_user", "user_auths"} {
		t.Run(table, func(t *testing.T) {
			db := openTestDB(t)
			mustCreateRole(t, db, "user", 1)
			kind := "create"
			if table == "sys_role" {
				kind = "query"
			}
			want, remove := injectRepoFailure(t, db, kind, table, 1)
			err := NewUserRepository(db).CreateAccount(&model.User{Username: "atomic", Password: "hash"}, &model.UserAuth{IdentityType: "password", Identifier: "atomic"})
			if !errors.Is(err, want) {
				t.Fatalf("error = %v", err)
			}
			remove()
			for _, table := range []string{"sys_user", "user_auths", "user_roles", "casbin_rule"} {
				var n int64
				if err := db.Table(table).Count(&n).Error; err != nil || n != 0 {
					t.Fatalf("partial %s: %d %v", table, n, err)
				}
			}
		})
	}
}

func TestRepositoryLookupAndListErrors(t *testing.T) {
	db := openTestDB(t)
	r := NewUserRepository(db)
	if _, err := r.GetUserRoles(9999); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("missing roles lookup: %v", err)
	}
	if err := r.UpdatePassword(9999, "new"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("missing password update: %v", err)
	}
	if err := r.CompleteBootstrapPassword(9999, "s", "old", "new"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("missing bootstrap update: %v", err)
	}
	if _, err := NewRoleRepository(db).GetRoleMenus(9999); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("missing role menus: %v", err)
	}
	want, _ := injectRepoFailure(t, db, "query", "", 1)
	if rows, total, err := r.ListWithKeyword("", 1, 10); !errors.Is(err, want) || rows != nil || total != 0 {
		t.Fatalf("user count failure: %+v %d %v", rows, total, err)
	}
}

func TestRoleCascadeRollbackAtEachStage(t *testing.T) {
	for _, table := range []string{"sys_role", "user_roles", "role_menus", "casbin_rule"} {
		t.Run(table, func(t *testing.T) {
			db := openTestDB(t)
			u := seedRepoAccount(t, db)
			roles, err := NewUserRepository(db).GetUserRoles(u.ID)
			if err != nil {
				t.Fatal(err)
			}
			role := roles[0]
			menu := model.Menu{Name: "kept"}
			db.Create(&menu)
			db.Create(&model.RoleMenu{RoleID: role.ID, MenuID: menu.ID})
			kind := "delete"
			if table == "sys_role" {
				kind = "query"
			}
			want, remove := injectRepoFailure(t, db, kind, table, 1)
			if err := NewRoleRepository(db).DeleteCascade(role.ID); !errors.Is(err, want) {
				t.Fatalf("cascade error = %v", err)
			}
			remove()
			for _, tbl := range []string{"sys_role", "user_roles", "role_menus", "casbin_rule"} {
				var n int64
				db.Table(tbl).Count(&n)
				if n != 1 {
					t.Fatalf("cascade failed to roll back %s: %d", tbl, n)
				}
			}
		})
	}
}

func TestOperationLogFiltersAndPagination(t *testing.T) {
	db := openTestDB(t)
	r := NewOperationLogRepository(db)
	base := time.Date(2026, 1, 2, 12, 0, 0, 0, time.Local)
	entries := []model.OperationLog{
		{Username: "alice", Module: "users", Method: "POST", Status: 1},
		{Username: "alice", Module: "users", Method: "GET", Status: 0},
		{Username: "bob", Module: "roles", Method: "POST", Status: 0},
	}
	for i := range entries {
		entries[i].CreatedAt = base.Add(time.Duration(i) * time.Hour)
		if err := r.Create(context.Background(), &entries[i]); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		name  string
		q     OpLogQuery
		ids   []uint64
		total int64
	}{
		{"combined", OpLogQuery{Username: "lic", Module: "users", Method: "POST", Status: 1}, []uint64{entries[0].ID}, 1},
		{"failed", OpLogQuery{Status: 2}, []uint64{entries[2].ID, entries[1].ID}, 2},
		{"time window", OpLogQuery{Start: "2026-01-02 13:00:00", End: "2026-01-02 13:00:00"}, []uint64{entries[1].ID}, 1},
		{"invalid dates ignored", OpLogQuery{Start: "bad", End: "bad"}, []uint64{entries[2].ID, entries[1].ID, entries[0].ID}, 3},
		{"second page", OpLogQuery{Page: 2, PageSize: 1}, []uint64{entries[1].ID}, 3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, total, err := r.List(context.Background(), c.q)
			if err != nil || total != c.total || len(got) != len(c.ids) {
				t.Fatalf("list = %+v %d %v", got, total, err)
			}
			for i, id := range c.ids {
				if got[i].ID != id {
					t.Fatalf("row %d = %d, want %d", i, got[i].ID, id)
				}
			}
		})
	}
}

func TestPagedQueriesPropagateCountAndFetchFailures(t *testing.T) {
	for _, repo := range []string{"logs", "permissions", "users"} {
		for nth := 1; nth <= 2; nth++ {
			t.Run(fmt.Sprintf("%s/%d", repo, nth), func(t *testing.T) {
				db := openTestDB(t)
				want, _ := injectRepoFailure(t, db, "query", "", nth)
				var err error
				switch repo {
				case "logs":
					_, _, err = NewOperationLogRepository(db).List(context.Background(), OpLogQuery{})
				case "permissions":
					_, _, err = NewPermissionRepository(db).ListWithFilter("", "", nil, 1, 10)
				case "users":
					_, _, err = NewUserRepository(db).ListWithKeyword("", 1, 10)
				}
				if !errors.Is(err, want) {
					t.Fatalf("paged error = %v", err)
				}
			})
		}
	}
}

func TestIdentityDeleteAndEnableAccount(t *testing.T) {
	db := openTestDB(t)
	u := seedRepoAccount(t, db)
	r := NewUserRepository(db)
	if err := r.UpdateStatus(u.ID, 1); err != nil {
		t.Fatal(err)
	}
	if err := NewAuthSessionRepository(db).Validate("rollback-session", u.ID); err != nil {
		t.Fatal("enabling revoked session")
	}
	ids, err := NewUserAuthRepository(db).ListByUserID(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := NewUserAuthRepository(db).Delete(ids[0].ID); err != nil {
		t.Fatal(err)
	}
	after, err := NewUserAuthRepository(db).ListByUserID(u.ID)
	if err != nil || len(after) != 0 {
		t.Fatalf("delete = %+v %v", after, err)
	}
}
