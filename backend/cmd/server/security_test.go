package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"sentinelgo/internal/model"
	"sentinelgo/internal/repository"
	"sentinelgo/internal/service"
	"sentinelgo/pkg/casbin"
	"sentinelgo/pkg/config"
	"sentinelgo/pkg/i18n"
	myoauth "sentinelgo/pkg/oauth"
	"sentinelgo/pkg/sms"
	"sentinelgo/pkg/token"
	"sentinelgo/pkg/utils"
)

type securityFixture struct {
	db          *gorm.DB
	router      http.Handler
	auth        *service.AuthService
	users       *service.UserService
	roles       *service.RoleService
	admin, user model.Role
	account     model.User
}

func newSecurityFixture(t *testing.T) *securityFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	gin.DefaultWriter = io.Discard
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })
	runMigrations(db)
	casbin.Init(db, "../../config/rbac_model.conf")
	config.GlobalManager.Update(&config.AppConfig{App: config.AppCfg{Env: "dev"}, SMS: config.SMSCfg{Provider: "dev"}, SystemSwitches: config.SystemSwitches{EnableRegister: true}})
	token.Init(strings.Repeat("test-key-", 5), nil)
	service.InitMultiAuth(sms.NewSMSService(nil, sms.NewDevSender(), sms.Config{}), myoauth.NewGitHubOAuth("test", "test", "http://localhost/callback", "http://localhost:3001", ""))
	f := &securityFixture{db: db, admin: model.Role{Code: "admin", Name: "Admin", Status: 1}, user: model.Role{Code: "user", Name: "User", Status: 1}}
	for _, r := range []*model.Role{&f.admin, &f.user} {
		if err := db.Create(r).Error; err != nil {
			t.Fatal(err)
		}
	}
	casbin.AddPolicy("admin", "*", "*")
	for _, p := range []struct{ path, method string }{
		{"/api/v1/user/info", "GET"}, {"/api/v1/user/profile", "GET"},
		{"/api/v1/auth/logout", "POST"}, {"/api/v1/auth/channels", "GET"},
		{"/api/v1/auth/bind/phone", "POST"}, {"/api/v1/auth/sms/send", "POST"},
	} {
		casbin.AddPolicy("user", p.path, p.method)
	}
	hash, _ := utils.HashPassword("ValidPassword123!")
	f.account = model.User{Username: "fixture", Password: hash, Status: 1, Roles: []model.Role{f.admin}}
	if err := db.Create(&f.account).Error; err != nil {
		t.Fatal(err)
	}
	casbin.AddRoleForUser(fmt.Sprint(f.account.ID), "admin")
	ur, rr := repository.NewUserRepository(db), repository.NewRoleRepository(db)
	f.auth = service.NewAuthService(ur, rr, db)
	f.users = service.NewUserService(ur, rr)
	f.roles = service.NewRoleService(rr, repository.NewMenuRepository(db))
	f.router = InitComponents(db)
	return f
}

type apiResult struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

func TestBootstrapPasswordCompletion(t *testing.T) {
	f := newSecurityFixture(t)
	initial := "TemporaryPassword123!"
	t.Setenv("KS_BOOTSTRAP_ADMIN_PASSWORD", initial)
	if err := f.auth.InitAdmin(); err != nil {
		t.Fatal(err)
	}
	login := f.request(t, "POST", "/api/v1/auth/login", "", map[string]string{"username": "admin", "password": initial})
	var data struct {
		Access   string `json:"access_token"`
		Refresh  string `json:"refresh_token"`
		Required bool   `json:"password_change_required"`
	}
	if err := json.Unmarshal(login.Data, &data); err != nil {
		t.Fatal(err)
	}
	if login.Code != 0 || !data.Required {
		t.Fatalf("initial login must require password change: %+v", login)
	}
	for _, path := range []string{"/api/v1/users", "/api/v1/user/info", "/api/v1/user/menus"} {
		if res := f.request(t, "GET", path, data.Access, nil); res.Code != 20003 {
			t.Fatalf("bootstrap access bypassed restriction: %+v", res)
		}
	}
	request := func(old, password string) apiResult {
		return f.request(t, "POST", "/api/v1/auth/password/initialize", data.Access, map[string]string{"current_password": old, "new_password": password, "confirm_password": password})
	}
	if res := request("wrong", "FormalPassword456!"); res.Code != 11000 {
		t.Fatalf("wrong current password accepted: %+v", res)
	}
	if res := request(initial, initial); res.Code != 11000 {
		t.Fatalf("initial password reuse accepted: %+v", res)
	}
	if res := request(initial, "short"); res.Code != 11000 {
		t.Fatalf("weak formal password accepted: %+v", res)
	}
	rotated := f.request(t, "POST", "/api/v1/auth/refresh", "", map[string]string{"refresh_token": data.Refresh})
	var pair token.TokenPair
	if err := json.Unmarshal(rotated.Data, &pair); err != nil {
		t.Fatal(err)
	}
	if res := f.request(t, "GET", "/api/v1/users", pair.AccessToken, nil); res.Code != 20003 {
		t.Fatalf("refresh bypassed bootstrap restriction: %+v", res)
	}
	if res := request(initial, "FormalPassword456!"); res.Code != 0 {
		t.Fatalf("formal password setup failed: %+v", res)
	}
	if res := f.request(t, "GET", "/api/v1/users", data.Access, nil); res.Code != 20000 {
		t.Fatalf("old access survived completion: %+v", res)
	}
	if res := f.request(t, "POST", "/api/v1/auth/refresh", "", map[string]string{"refresh_token": pair.RefreshToken}); res.Code != 20001 {
		t.Fatalf("old refresh survived completion: %+v", res)
	}
	if _, _, err := f.auth.Login("admin", initial); err == nil {
		t.Fatal("temporary password still works")
	}
	formal, _, err := f.auth.Login("admin", "FormalPassword456!")
	if err != nil {
		t.Fatal(err)
	}
	if res := f.request(t, "GET", "/api/v1/users", formal.AccessToken, nil); res.Code != 0 {
		t.Fatalf("formal login cannot access admin: %+v", res)
	}
	if err := f.auth.InitAdmin(); err != nil {
		t.Fatal(err)
	}
	if res := f.request(t, "GET", "/api/v1/users", formal.AccessToken, nil); res.Code != 0 {
		t.Fatalf("restart reinstated requirement: %+v", res)
	}
}

func TestLegacyBootstrapPasswordRequiresInitialization(t *testing.T) {
	f := newSecurityFixture(t)
	if err := f.db.Model(&f.account).Updates(map[string]any{"username": "admin", "is_admin": true}).Error; err != nil {
		t.Fatal(err)
	}
	t.Setenv("KS_BOOTSTRAP_ADMIN_PASSWORD", "ValidPassword123!")
	if err := f.auth.InitAdmin(); err != nil {
		t.Fatal(err)
	}
	_, user, err := f.auth.Login("admin", "ValidPassword123!")
	if err != nil || !user.PasswordChangeRequired {
		t.Fatalf("legacy temporary password not restricted: %v", err)
	}
}

func TestLegacyMigrationDoesNotOverwriteConcurrentCompletion(t *testing.T) {
	f := newSecurityFixture(t)
	if err := f.db.Model(&f.account).Updates(map[string]any{"username": "admin", "is_admin": true}).Error; err != nil {
		t.Fatal(err)
	}
	t.Setenv("KS_BOOTSTRAP_ADMIN_PASSWORD", "ValidPassword123!")
	formalHash, _ := utils.HashPassword("ConcurrentFormalPassword123!")
	if err := f.db.Callback().Update().Before("gorm:update").Register("simulate_bootstrap_completion", func(tx *gorm.DB) {
		if tx.Statement.Table == "sys_user" {
			if err := tx.Exec("UPDATE sys_user SET password = ?, bootstrap_completed = ?, password_change_required = ? WHERE id = ?", formalHash, true, false, f.account.ID).Error; err != nil {
				tx.AddError(err)
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	if err := f.auth.InitAdmin(); err != nil {
		t.Fatal(err)
	}
	_, user, err := f.auth.Login("admin", "ConcurrentFormalPassword123!")
	if err != nil || user.PasswordChangeRequired || !user.BootstrapCompleted {
		t.Fatalf("migration overwrote concurrent completion: %v", err)
	}
}

func (f *securityFixture) request(t *testing.T, method, path, access string, body any) apiResult {
	t.Helper()
	var data []byte
	if body != nil {
		var err error
		data, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(data))
	r.Header.Set("Content-Type", "application/json")
	if access != "" {
		r.Header.Set("Authorization", "Bearer "+access)
	}
	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, r)
	var result apiResult
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("invalid response %q: %v", w.Body.String(), err)
	}
	return result
}

func (f *securityFixture) login(t *testing.T) *token.TokenPair {
	t.Helper()
	pair, _, err := f.auth.Login("fixture", "ValidPassword123!")
	if err != nil {
		t.Fatal(err)
	}
	return pair
}

func TestDowngradeRevokesAdminPermissions(t *testing.T) {
	f := newSecurityFixture(t)
	pair := f.login(t)
	if err := f.users.AssignRoles(f.account.ID, []uint64{f.user.ID}); err != nil {
		t.Fatal(err)
	}
	if res := f.request(t, "GET", "/api/v1/users", pair.AccessToken, nil); res.Code != 20002 {
		t.Fatalf("downgraded account retained admin access: %+v", res)
	}
	roles, _ := casbin.GetRolesForUser(fmt.Sprint(f.account.ID))
	for _, role := range roles {
		if role == "admin" {
			t.Fatal("stale admin mapping remains")
		}
	}
}

func TestDisabledAccountRevokesExistingSession(t *testing.T) {
	f := newSecurityFixture(t)
	pair := f.login(t)
	if err := f.users.UpdateStatus(f.account.ID, 0); err != nil {
		t.Fatal(err)
	}
	if res := f.request(t, "GET", "/api/v1/users", pair.AccessToken, nil); res.Code == 0 {
		t.Fatal("disabled account accepted")
	}
	if _, err := f.auth.RefreshAccessToken(pair.RefreshToken); err == nil {
		t.Fatal("disabled account refreshed")
	}
	if err := f.users.UpdateStatus(f.account.ID, 1); err != nil {
		t.Fatal(err)
	}
	if res := f.request(t, "GET", "/api/v1/users", pair.AccessToken, nil); res.Code == 0 {
		t.Fatal("reenabling revived revoked session")
	}
}

func TestDeletedAccountCannotUseExistingAccessToken(t *testing.T) {
	f := newSecurityFixture(t)
	pair := f.login(t)
	if err := f.users.Delete(f.account.ID); err != nil {
		t.Fatal(err)
	}
	if res := f.request(t, "GET", "/api/v1/users", pair.AccessToken, nil); res.Code == 0 {
		t.Fatal("deleted account accepted")
	}
}

func TestDisabledRoleRevokesExistingAccess(t *testing.T) {
	f := newSecurityFixture(t)
	pair := f.login(t)
	if err := f.roles.UpdateStatus(f.admin.ID, 0); err != nil {
		t.Fatal(err)
	}
	if res := f.request(t, "GET", "/api/v1/users", pair.AccessToken, nil); res.Code != 20002 {
		t.Fatalf("disabled role retained access: %+v", res)
	}
}

func TestDeletedRoleRemovesPoliciesAndAccess(t *testing.T) {
	f := newSecurityFixture(t)
	pair := f.login(t)
	if err := f.roles.Delete(f.admin.ID); err != nil {
		t.Fatal(err)
	}
	if res := f.request(t, "GET", "/api/v1/users", pair.AccessToken, nil); res.Code != 20002 {
		t.Fatalf("deleted role retained access: %+v", res)
	}
	var count int64
	f.db.Model(&model.CasbinRule{}).Where("ptype = ? AND v0 = ?", "p", "admin").Count(&count)
	if count != 0 {
		t.Fatal("deleted role policies remain in database")
	}
}

func TestLogoutRevokesOnlyCurrentSession(t *testing.T) {
	f := newSecurityFixture(t)
	pair, other := f.login(t), f.login(t)
	if res := f.request(t, "POST", "/api/v1/auth/logout", pair.AccessToken, nil); res.Code != 0 {
		t.Fatalf("logout failed: %+v", res)
	}
	if _, err := f.auth.RefreshAccessToken(pair.RefreshToken); err == nil {
		t.Fatal("logged-out session refreshed")
	}
	if res := f.request(t, "GET", "/api/v1/user/info", pair.AccessToken, nil); res.Code == 0 {
		t.Fatal("logged-out access token accepted")
	}
	if res := f.request(t, "GET", "/api/v1/user/info", other.AccessToken, nil); res.Code != 0 {
		t.Fatalf("other session revoked: %+v", res)
	}
}

func TestRefreshTokenIsSingleUse(t *testing.T) {
	f := newSecurityFixture(t)
	pair := f.login(t)
	rotated, err := f.auth.RefreshAccessToken(pair.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.auth.RefreshAccessToken(pair.RefreshToken); err == nil {
		t.Fatal("refresh token replay accepted")
	}
	if res := f.request(t, "GET", "/api/v1/user/info", rotated.AccessToken, nil); res.Code != 0 {
		t.Fatalf("new access token rejected: %+v", res)
	}
}

func TestRegistrationInitializesPermissionsAndPasswordChannel(t *testing.T) {
	f := newSecurityFixture(t)
	user, err := f.auth.Register("registered", "ValidPassword123!", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	pair, _, err := f.auth.Login("registered", "ValidPassword123!")
	if err != nil {
		t.Fatal(err)
	}
	if res := f.request(t, "GET", "/api/v1/user/info", pair.AccessToken, nil); res.Code != 0 {
		t.Fatalf("registered user lacks basic access: %+v", res)
	}
	channels, err := f.auth.GetUserChannels(user.ID)
	if err != nil || len(channels) != 1 || channels[0].IdentityType != "password" {
		t.Fatalf("password channel missing: %+v %v", channels, err)
	}
}

func TestBindSMSUsesSeparateScene(t *testing.T) {
	f := newSecurityFixture(t)
	pair := f.login(t)
	res := f.request(t, "POST", "/api/v1/auth/sms/send", pair.AccessToken, map[string]string{"phone": "13800000001", "scene": "bind"})
	if res.Code != 0 {
		t.Fatalf("send failed: %+v", res)
	}
	var data struct {
		Code string `json:"dev_code"`
	}
	json.Unmarshal(res.Data, &data)
	if res := f.request(t, "POST", "/api/v1/auth/bind/phone", pair.AccessToken, map[string]string{"phone": "13800000001", "code": data.Code}); res.Code != 0 {
		t.Fatalf("binding valid SMS failed: %+v", res)
	}
}

func TestOAuthRejectsMissingStateBeforeExchangingCode(t *testing.T) {
	f := newSecurityFixture(t)
	res := f.request(t, "GET", "/api/v1/auth/oauth/github/callback?code=attacker-code", "", nil)
	if res.Code != 11000 {
		t.Fatalf("missing state not rejected: %+v", res)
	}
}

func TestPasswordChangeRevokesSessionsAndRejectsOversizedPasswords(t *testing.T) {
	f := newSecurityFixture(t)
	pair := f.login(t)
	if err := f.users.UpdatePassword(f.account.ID, strings.Repeat("a", 73)); err == nil {
		t.Fatal("oversized password accepted")
	}
	if _, _, err := f.auth.Login("fixture", "ValidPassword123!"); err != nil {
		t.Fatal("failed update damaged existing password")
	}
	if err := f.users.UpdatePassword(f.account.ID, "ChangedPassword123!"); err != nil {
		t.Fatal(err)
	}
	if res := f.request(t, "GET", "/api/v1/users", pair.AccessToken, nil); res.Code == 0 {
		t.Fatal("password change retained old session")
	}
	if _, err := f.auth.RefreshAccessToken(pair.RefreshToken); err == nil {
		t.Fatal("old refresh survived password change")
	}
}

func TestAdminCreatedUserUsesRequestedPassword(t *testing.T) {
	f := newSecurityFixture(t)
	pair := f.login(t)
	res := f.request(t, "POST", "/api/v1/users", pair.AccessToken, map[string]string{"username": "created", "password": "RequestedPassword123!"})
	if res.Code != 0 {
		t.Fatalf("create failed: %+v", res)
	}
	if _, _, err := f.auth.Login("created", "RequestedPassword123!"); err != nil {
		t.Fatalf("requested password was ignored: %v", err)
	}
	if _, _, err := f.auth.Login("created", "123456"); err == nil {
		t.Fatal("default weak password still accepted")
	}
}

func TestRestartDoesNotRestoreRevokedRoles(t *testing.T) {
	f := newSecurityFixture(t)
	f.db.Model(&model.User{}).Where("id = ?", f.account.ID).Update("username", "admin")
	if err := f.users.AssignRoles(f.account.ID, nil); err != nil {
		t.Fatal(err)
	}
	if err := f.auth.InitAdmin(); err != nil {
		t.Fatal(err)
	}
	roles, err := repository.NewUserRepository(f.db).GetUserRoles(f.account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != 0 {
		t.Fatal("startup restored explicitly revoked roles")
	}
}

func TestRestartDoesNotRestoreRevokedRolePermissions(t *testing.T) {
	f := newSecurityFixture(t)
	t.Setenv("KS_BOOTSTRAP_ADMIN_PASSWORD", "BootstrapPassword123!")
	if _, err := casbin.DeletePermissionsForUser("admin"); err != nil {
		t.Fatal(err)
	}
	if err := f.auth.InitAdmin(); err != nil {
		t.Fatal(err)
	}
	allowed, err := casbin.CheckPermission("admin", "/api/v1/users", "GET")
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatal("startup restored revoked role permissions")
	}
}

func TestResetPasswordCreatesPasswordLoginChannel(t *testing.T) {
	f := newSecurityFixture(t)
	code, err := f.auth.SendSMSCode("13800000002", i18n.ZhCN)
	if err != nil {
		t.Fatal(err)
	}
	_, user, err := f.auth.LoginBySMS("13800000002", code)
	if err != nil {
		t.Fatal(err)
	}
	resetCode, err := f.auth.ForgotPassword("13800000002", i18n.ZhCN)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.auth.ResetPassword("13800000002", resetCode, "ChangedPassword123!"); err != nil {
		t.Fatal(err)
	}
	channels, err := f.auth.GetUserChannels(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, channel := range channels {
		if channel.IdentityType == "password" {
			return
		}
	}
	t.Fatal("password reset enabled login but did not create password channel")
}

func TestMigrationRejectsDuplicateAuthenticationIdentities(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	defer sqlDB.Close()
	if err := db.Exec("CREATE TABLE user_auths (id integer PRIMARY KEY, user_id integer, identity_type text, identifier text, credential text, created_at datetime, updated_at datetime)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO user_auths (id,user_id,identity_type,identifier) VALUES (1,1,'phone','13800000001'),(2,2,'phone','13800000001')").Error; err != nil {
		t.Fatal(err)
	}
	defer func() {
		if recover() == nil {
			t.Error("migration failure did not prevent startup")
		}
	}()
	runMigrations(db)
}
