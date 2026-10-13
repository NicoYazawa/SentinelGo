package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"sentinelgo/internal/model"
	"sentinelgo/internal/repository"
	"sentinelgo/pkg/config"
	"sentinelgo/pkg/i18n"
	"sentinelgo/pkg/sms"
	"sentinelgo/pkg/token"
)

// All injected failures use GORM's public callbacks and stay confined to this DB.
func failServiceDB(t *testing.T, db *gorm.DB, kind, table string, nth int) error {
	t.Helper()
	sentinel := errors.New("injected database failure")
	calls := 0
	callback := func(tx *gorm.DB) {
		if table == "" || tx.Statement.Table == table {
			calls++
			if calls == nth {
				tx.AddError(sentinel)
			}
		}
	}
	name := "test:service_failure"
	var err error
	switch kind {
	case "query":
		err = db.Callback().Query().Before("gorm:query").Register(name, callback)
		t.Cleanup(func() { db.Callback().Query().Remove(name) })
	case "create":
		err = db.Callback().Create().Before("gorm:create").Register(name, callback)
		t.Cleanup(func() { db.Callback().Create().Remove(name) })
	case "update":
		err = db.Callback().Update().Before("gorm:update").Register(name, callback)
		t.Cleanup(func() { db.Callback().Update().Remove(name) })
	case "delete":
		err = db.Callback().Delete().Before("gorm:delete").Register(name, callback)
		t.Cleanup(func() { db.Callback().Delete().Remove(name) })
	}
	if err != nil {
		t.Fatal(err)
	}
	return sentinel
}

func TestAuthUnconfiguredDependencies(t *testing.T) {
	_, svc := setupService(t)
	InitMultiAuth(nil, nil)
	checks := []struct {
		name string
		run  func() error
	}{
		{"send", func() error { _, e := svc.SendSMSCode("13811110001", i18n.ZhCN); return e }},
		{"login", func() error { _, _, e := svc.LoginBySMS("13811110001", "x"); return e }},
		{"bind", func() error { return svc.BindPhone(1, "13811110001", "x") }},
		{"forgot", func() error { _, e := svc.ForgotPassword("13811110001", i18n.ZhCN); return e }},
		{"reset", func() error { return svc.ResetPassword("13811110001", "x", "new") }},
		{"oauth URL", func() error { _, _, e := svc.GetGitHubAuthURL(); return e }},
		{"oauth login", func() error { _, _, e := svc.LoginByGitHub("x"); return e }},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			if c.run() == nil {
				t.Fatal("unconfigured dependency accepted")
			}
		})
	}
	if got := svc.GetFrontendURL(); got != "http://localhost:3001" {
		t.Fatalf("fallback = %q", got)
	}
}

func TestAuthSessionLifecycleEdges(t *testing.T) {
	db, svc := setupService(t)
	u := mustCreateUser(t, db, "edge-session", "ValidPassword123!", 1)
	pair, _, err := svc.Login(u.Username, "ValidPassword123!")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := token.ParseToken(pair.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if err = svc.ValidateAccessToken("invalid"); err == nil {
		t.Fatal("invalid JWT accepted")
	}
	if err = svc.ValidateAccessToken(pair.RefreshToken); !errors.Is(err, repository.ErrSessionInvalid) {
		t.Fatalf("refresh validated as access: %v", err)
	}
	if _, err = svc.RefreshAccessToken("invalid"); err == nil {
		t.Fatal("invalid refresh accepted")
	}
	rotated, err := svc.RefreshAccessToken(pair.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.RefreshAccessToken(pair.RefreshToken); !errors.Is(err, repository.ErrSessionInvalid) {
		t.Fatalf("refresh replay: %v", err)
	}
	if err = svc.Logout(u.ID, claims.SessionID); err != nil {
		t.Fatal(err)
	}
	if err = svc.ValidateAccessToken(rotated.AccessToken); !errors.Is(err, repository.ErrSessionInvalid) {
		t.Fatalf("revoked access accepted: %v", err)
	}
	if _, err = svc.RefreshAccessToken(rotated.RefreshToken); !errors.Is(err, repository.ErrSessionInvalid) {
		t.Fatalf("revoked refresh accepted: %v", err)
	}
}

func TestAuthSessionRejectsStaleAccount(t *testing.T) {
	for _, change := range []string{"password", "status", "deleted"} {
		t.Run(change, func(t *testing.T) {
			db, svc := setupService(t)
			u := mustCreateUser(t, db, "stale", "ValidPassword123!", 1)
			switch change {
			case "password":
				db.Model(&model.User{}).Where("id = ?", u.ID).Update("password", "changed")
			case "status":
				db.Model(&model.User{}).Where("id = ?", u.ID).Update("status", 0)
			case "deleted":
				db.Delete(&u)
			}
			if _, err := svc.newSession(&u, nil); err == nil {
				t.Fatal("stale account received session")
			}
			var n int64
			db.Model(&model.AuthSession{}).Count(&n)
			if n != 0 {
				t.Fatalf("sessions = %d", n)
			}
		})
	}
}

func TestAuthValidationAccountChanges(t *testing.T) {
	for _, change := range []string{"disabled", "deleted"} {
		t.Run(change, func(t *testing.T) {
			db, svc := setupService(t)
			u := mustCreateUser(t, db, "changed", "ValidPassword123!", 1)
			pair, _, err := svc.Login(u.Username, "ValidPassword123!")
			if err != nil {
				t.Fatal(err)
			}
			if change == "disabled" {
				db.Model(&model.User{}).Where("id = ?", u.ID).Update("status", 0)
			} else {
				db.Delete(&u)
			}
			if err := svc.ValidateAccessToken(pair.AccessToken); err == nil {
				t.Fatal("changed account access accepted")
			}
			if _, err := svc.RefreshAccessToken(pair.RefreshToken); err == nil {
				t.Fatal("changed account refresh accepted")
			}
		})
	}
}

func TestAuthPersistenceErrors(t *testing.T) {
	cases := []struct {
		name, kind, table string
		run               func(*AuthService) error
	}{
		{"login lookup", "query", "sys_user", func(s *AuthService) error { _, _, e := s.Login("failure", "ValidPassword123!"); return e }},
		{"login roles", "query", "sys_role", func(s *AuthService) error { _, _, e := s.Login("failure", "ValidPassword123!"); return e }},
		{"login session", "create", "auth_sessions", func(s *AuthService) error { _, _, e := s.Login("failure", "ValidPassword123!"); return e }},
		{"register lookup", "query", "sys_user", func(s *AuthService) error { _, e := s.Register("fresh", "ValidPassword123!", "", "", ""); return e }},
		{"register write", "create", "sys_user", func(s *AuthService) error { _, e := s.Register("fresh", "ValidPassword123!", "", "", ""); return e }},
		{"bootstrap lookup", "query", "sys_user", func(s *AuthService) error { _, _, e := s.BootstrapStatus(); return e }},
		{"oauth state insert", "create", "o_auth_states", func(s *AuthService) error { _, _, e := s.GetGitHubAuthURL(); return e }},
		{"oauth state delete", "delete", "o_auth_states", func(s *AuthService) error { return s.ConsumeOAuthState("state") }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			db, svc := setupService(t)
			mustCreateUser(t, db, "failure", "ValidPassword123!", 1)
			want := failServiceDB(t, db, c.kind, c.table, 1)
			if err := c.run(svc); !errors.Is(err, want) {
				t.Fatalf("error = %v, want injected error", err)
			}
		})
	}
	_, svc := setupService(t)
	if _, err := svc.Register("long", strings.Repeat("a", 73), "", "", ""); err == nil {
		t.Fatal("bcrypt byte limit ignored")
	}
	if err := svc.CompleteBootstrapPassword(99999, "s", "old", "ValidPassword123!"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("missing bootstrap account = %v", err)
	}
}

func TestBootstrapStatusAndLegacyUpgrade(t *testing.T) {
	db, svc := setupService(t)
	u := mustCreateUser(t, db, "admin", "TemporaryPassword123!", 1)
	t.Setenv("KS_BOOTSTRAP_ADMIN_PASSWORD", "TemporaryPassword123!")
	if err := svc.InitAdmin(); err != nil {
		t.Fatal(err)
	}
	required, completed, err := svc.BootstrapStatus()
	if err != nil || !required || completed {
		t.Fatalf("status = %v %v %v", required, completed, err)
	}
	var roles int64
	db.Model(&model.UserRole{}).Where("user_id = ?", u.ID).Count(&roles)
	if roles != 0 {
		t.Fatal("upgrade regranted revoked admin role")
	}
	channels, err := svc.GetUserChannels(u.ID)
	if err != nil || len(channels) != 1 || channels[0].Credential != u.Password {
		t.Fatalf("password metadata repair: %+v %v", channels, err)
	}
}

func smsEdgeCode(t *testing.T, phone, scene string) string {
	t.Helper()
	code, err := SMSSvc.SendCode(context.Background(), scene, phone, i18n.ZhCN)
	if err != nil {
		t.Fatal(err)
	}
	return code
}

func TestSMSAccountEdges(t *testing.T) {
	for _, mode := range []string{"disabled", "missing", "collision", "closed", "database", "create"} {
		t.Run(mode, func(t *testing.T) {
			db, svc := setupService(t)
			phone := "13811112222"
			if mode == "disabled" || mode == "missing" {
				u := mustCreateUser(t, db, "sms-edge", "ValidPassword123!", 0)
				db.Create(&model.UserAuth{UserID: u.ID, IdentityType: "phone", Identifier: phone})
				if mode == "missing" {
					db.Delete(&u)
				}
			}
			if mode == "collision" {
				mustCreateUser(t, db, "u_"+phone, "ValidPassword123!", 1)
			}
			if mode == "closed" {
				cfg := *config.GlobalManager.Get()
				cfg.SystemSwitches.EnableRegister = false
				config.GlobalManager.Update(&cfg)
			}
			code := smsEdgeCode(t, phone, sms.SceneLogin)
			if mode == "database" {
				failServiceDB(t, db, "query", "user_auths", 1)
			}
			if mode == "create" {
				failServiceDB(t, db, "create", "sys_user", 1)
			}
			pair, user, err := svc.LoginBySMS(phone, code)
			if mode == "collision" {
				if err != nil || user.Username != "u_"+phone+"_1" || pair.AccessToken == "" {
					t.Fatalf("collision login = %+v %v", user, err)
				}
			} else if err == nil {
				t.Fatal("invalid SMS account accepted")
			}
		})
	}
}

func TestPhoneRecoveryAndBindingEdges(t *testing.T) {
	db, svc := setupService(t)
	if _, err := svc.ForgotPassword("13811113333", i18n.ZhCN); err == nil {
		t.Fatal("unregistered phone accepted")
	}
	phone := "13811113334"
	code := smsEdgeCode(t, phone, sms.SceneReset)
	if err := svc.ResetPassword(phone, code, "ValidPassword123!"); err == nil {
		t.Fatal("unbound reset accepted")
	}
	phone = "13811113335"
	u := mustCreateUser(t, db, "recover", "ValidPassword123!", 1)
	db.Create(&model.UserAuth{UserID: u.ID, IdentityType: "phone", Identifier: phone})
	code = smsEdgeCode(t, phone, sms.SceneReset)
	if err := svc.ResetPassword(phone, code, strings.Repeat("a", 73)); err == nil {
		t.Fatal("oversize reset password accepted")
	}
	phone = "13811113336"
	db.Create(&model.UserAuth{UserID: u.ID, IdentityType: "phone", Identifier: phone})
	code = smsEdgeCode(t, phone, sms.SceneBind)
	if err := svc.BindPhone(u.ID, phone, code); err == nil || !strings.Contains(err.Error(), "已绑定") {
		t.Fatalf("duplicate binding = %v", err)
	}
}

func TestExpiredOAuthState(t *testing.T) {
	db, svc := setupService(t)
	db.Create(&model.OAuthState{ID: "expired", ExpiresAt: time.Now().Add(-time.Second)})
	if err := svc.ConsumeOAuthState("expired"); err == nil {
		t.Fatal("expired state accepted")
	}
}

func TestGitHubAccountEdges(t *testing.T) {
	for _, mode := range []string{"existing", "disabled", "missing", "nickname", "closed", "exchange", "userinfo", "lookup", "create", "avatar", "roles", "session"} {
		t.Run(mode, func(t *testing.T) {
			db, svc := setupService(t)
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if strings.Contains(r.URL.Path, "access_token") {
					if mode == "exchange" {
						w.WriteHeader(500)
						return
					}
					w.Write([]byte(`{"access_token":"test"}`))
					return
				}
				if mode == "userinfo" {
					w.WriteHeader(500)
					return
				}
				name := "GitHub User"
				if mode == "nickname" {
					name = ""
				}
				w.Write([]byte(`{"id":12345,"login":"gh-edge","name":"` + name + `","avatar_url":"https://example.test/new.png"}`))
			}))
			defer ts.Close()
			GitHubOAuth = newTestGitHubOAuth(ts.URL)
			var existing model.User
			if mode == "existing" || mode == "disabled" || mode == "missing" || mode == "avatar" || mode == "roles" || mode == "session" {
				existing = mustCreateUser(t, db, "github-edge", "ValidPassword123!", 1)
				mustAssignRole(t, db, existing.ID, "user")
				db.Create(&model.UserAuth{UserID: existing.ID, IdentityType: "github", Identifier: "github_12345"})
				if mode == "disabled" {
					db.Model(&model.User{}).Where("id = ?", existing.ID).Update("status", 0)
				}
				if mode == "missing" {
					db.Delete(&existing)
				}
			}
			switch mode {
			case "closed":
				cfg := *config.GlobalManager.Get()
				cfg.SystemSwitches.EnableRegister = false
				config.GlobalManager.Update(&cfg)
			case "lookup":
				failServiceDB(t, db, "query", "user_auths", 1)
			case "create":
				failServiceDB(t, db, "create", "sys_user", 1)
			case "avatar":
				failServiceDB(t, db, "update", "sys_user", 1)
			case "roles":
				failServiceDB(t, db, "query", "sys_role", 1)
			case "session":
				failServiceDB(t, db, "create", "auth_sessions", 1)
			}
			pair, user, err := svc.LoginByGitHub("test-code")
			if mode != "existing" && mode != "nickname" {
				if err == nil {
					t.Fatal("GitHub failure accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if pair.AccessToken == "" || user.Avatar != "https://example.test/new.png" {
				t.Fatalf("GitHub result = %+v", user)
			}
			if mode == "nickname" && user.Nickname != "gh-edge" {
				t.Fatalf("nickname fallback = %q", user.Nickname)
			}
			if mode == "existing" {
				if user.ID != existing.ID {
					t.Fatal("existing identity created duplicate user")
				}
				claims, e := token.ParseToken(pair.AccessToken)
				if e != nil || len(claims.Roles) != 1 || claims.Roles[0] != "user" {
					t.Fatalf("roles = %+v %v", claims, e)
				}
				got, e := svc.userRepo.FindForAuth(user.ID)
				if e != nil || got.Avatar != user.Avatar {
					t.Fatalf("avatar persistence = %+v %v", got, e)
				}
			}
		})
	}
}

func TestDashboardPropagatesEachQueryFailure(t *testing.T) {
	for nth := 1; nth <= 11; nth++ {
		t.Run(string(rune('A'+nth-1)), func(t *testing.T) {
			db, _ := setupService(t)
			svc := NewDashboardService(repository.NewDashboardRepository(db))
			want := failServiceDB(t, db, "query", "", nth)
			data, err := svc.Stats()
			if !errors.Is(err, want) || data != nil {
				t.Fatalf("failure %d returned %+v, %v", nth, data, err)
			}
		})
	}
}

func TestCRUDServicesRejectMissingAndUnwritableRecords(t *testing.T) {
	db, _ := setupService(t)
	users := NewUserService(repository.NewUserRepository(db), repository.NewRoleRepository(db))
	roles := NewRoleService(repository.NewRoleRepository(db), repository.NewMenuRepository(db))
	menus := NewMenuService(repository.NewMenuRepository(db), repository.NewRoleRepository(db), db)
	permissions := NewPermissionService(repository.NewPermissionRepository(db), db)
	missing := []func() error{
		func() error { return users.Update(99999, &model.User{Nickname: "missing"}) },
		func() error { return users.AssignRoles(99999, []uint64{1}) },
		func() error { return roles.Update(99999, map[string]interface{}{"name": "missing"}) },
		func() error { return roles.Delete(99999) },
		func() error { return roles.AssignPermissions(99999, nil) },
		func() error { return menus.Update(99999, &model.Menu{Name: "missing"}) },
		func() error { return permissions.Update(99999, &model.Permission{Path: "/missing"}) },
	}
	for i, run := range missing {
		if err := run(); err == nil {
			t.Fatalf("missing record operation %d succeeded", i)
		}
	}
	if err := users.Create(&model.User{Username: "long", Password: strings.Repeat("x", 73)}); err == nil {
		t.Fatal("long create password accepted")
	}
	u := mustCreateUser(t, db, "crud-edge", "ValidPassword123!", 1)
	if err := users.Update(u.ID, &model.User{Password: strings.Repeat("x", 73)}); err == nil {
		t.Fatal("long update password accepted")
	}
	want := failServiceDB(t, db, "update", "auth_sessions", 1)
	if err := users.Delete(u.ID); !errors.Is(err, want) {
		t.Fatalf("delete storage failure = %v", err)
	}
}
