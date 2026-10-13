package service

import (
	"testing"
	"time"

	"sentinelgo/internal/model"
	"sentinelgo/pkg/config"
	"sentinelgo/pkg/i18n"
	"sentinelgo/pkg/sms"
	"sentinelgo/pkg/utils"
)

func TestLoginSuccess(t *testing.T) {
	db, svc := setupService(t)
	u := mustCreateUser(t, db, "alice", "ValidPassword123!", 1)
	mustAssignRole(t, db, u.ID, "user")
	pair, got, err := svc.Login("alice", "ValidPassword123!")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != u.ID {
		t.Errorf("user mismatch")
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Error("tokens empty")
	}
}

func TestLoginWrongPassword(t *testing.T) {
	db, svc := setupService(t)
	u := mustCreateUser(t, db, "bob", "ValidPassword123!", 1)
	mustAssignRole(t, db, u.ID, "user")
	if _, _, err := svc.Login("bob", "wrong"); err == nil {
		t.Error("wrong password should fail")
	}
}

func TestLoginDisabled(t *testing.T) {
	db, svc := setupService(t)
	mustCreateUser(t, db, "carol", "ValidPassword123!", 0)
	if _, _, err := svc.Login("carol", "ValidPassword123!"); err == nil {
		t.Error("disabled user should fail")
	}
}

func TestLoginNotFound(t *testing.T) {
	_, svc := setupService(t)
	if _, _, err := svc.Login("nobody", "x"); err == nil {
		t.Error("nonexistent user should fail")
	}
}

func TestCompleteBootstrapPassword(t *testing.T) {
	db, svc := setupService(t)
	temp := "TemporaryPassword123!"
	hash, _ := utils.HashPassword(temp)
	u := model.User{Username: "boot", Password: hash, Status: 1, PasswordChangeRequired: true, Locale: "zh-CN"}
	db.Create(&u)
	db.Create(&model.AuthSession{ID: "sess-boot", UserID: u.ID, RefreshJTI: "jti", ExpiresAt: time.Now().Add(time.Hour)})

	// weak password
	if err := svc.CompleteBootstrapPassword(u.ID, "s", temp, "short"); err == nil {
		t.Error("weak password accepted")
	}
	// same as temp
	if err := svc.CompleteBootstrapPassword(u.ID, "s", temp, temp); err == nil {
		t.Error("reuse of temp accepted")
	}
	// wrong current
	if err := svc.CompleteBootstrapPassword(u.ID, "s", "wrongcurrent", "FormalPassword456!"); err == nil {
		t.Error("wrong current accepted")
	}
	// success
	if err := svc.CompleteBootstrapPassword(u.ID, "sess-boot", temp, "FormalPassword456!"); err != nil {
		t.Fatal(err)
	}
	var after model.User
	db.First(&after, u.ID)
	if !after.BootstrapCompleted {
		t.Error("bootstrap not completed")
	}
	// already completed
	if err := svc.CompleteBootstrapPassword(u.ID, "s", temp, "FormalPassword457!"); err == nil {
		t.Error("re-completion accepted")
	}
}

func TestBootstrapStatus(t *testing.T) {
	_, svc := setupService(t)
	required, completed, err := svc.BootstrapStatus()
	if err != nil {
		t.Fatal(err)
	}
	if required || completed {
		t.Error("fresh db should have no bootstrap admin")
	}
}

func TestRefreshAndValidate(t *testing.T) {
	db, svc := setupService(t)
	u := mustCreateUser(t, db, "dave", "ValidPassword123!", 1)
	mustAssignRole(t, db, u.ID, "user")
	pair, _, err := svc.Login("dave", "ValidPassword123!")
	if err != nil {
		t.Fatal(err)
	}
	// validate access
	if err := svc.ValidateAccessToken(pair.AccessToken); err != nil {
		t.Errorf("validate access failed: %v", err)
	}
	// refresh
	newPair, err := svc.RefreshAccessToken(pair.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	if newPair.AccessToken == "" {
		t.Error("refresh produced no token")
	}
	// refresh with access token should fail
	if _, err := svc.RefreshAccessToken(pair.AccessToken); err == nil {
		t.Error("access token used as refresh should fail")
	}
	// disabled user cannot refresh
	db.Model(&model.User{}).Where("id = ?", u.ID).Update("status", 0)
	if _, err := svc.RefreshAccessToken(newPair.RefreshToken); err == nil {
		t.Error("disabled user refresh should fail")
	}
}

func TestLogout(t *testing.T) {
	_, svc := setupService(t)
	if err := svc.Logout(1, ""); err == nil {
		t.Error("empty session should fail")
	}
}

func TestRegister(t *testing.T) {
	_, svc := setupService(t)
	user, err := svc.Register("newuser", "ValidPassword123!", "New", "n@x.com", "13800000000")
	if err != nil {
		t.Fatal(err)
	}
	if user.ID == 0 {
		t.Error("user not created")
	}
	// duplicate
	if _, err := svc.Register("newuser", "ValidPassword123!", "New", "", ""); err == nil {
		t.Error("duplicate username accepted")
	}
	// register disabled
	orig := config.GlobalManager.Get()
	defer config.GlobalManager.Update(orig)
	disabled := *orig
	disabled.SystemSwitches.EnableRegister = false
	config.GlobalManager.Update(&disabled)
	if _, err := svc.Register("closed", "ValidPassword123!", "C", "", ""); err == nil {
		t.Error("register when disabled should fail")
	}
}

func TestInitAdmin(t *testing.T) {
	db, svc := setupService(t)
	t.Setenv("KS_BOOTSTRAP_ADMIN_PASSWORD", "AtLeast12Bytes!!")
	if err := svc.InitAdmin(); err != nil {
		t.Fatal(err)
	}
	// idempotent
	if err := svc.InitAdmin(); err != nil {
		t.Fatal(err)
	}
	var admin model.User
	if err := db.Where("username = ?", "admin").First(&admin).Error; err != nil {
		t.Fatal(err)
	}
	if !admin.IsAdmin {
		t.Error("admin flag not set")
	}
}

func TestInitAdminMissingEnv(t *testing.T) {
	_, svc := setupService(t)
	t.Setenv("KS_BOOTSTRAP_ADMIN_PASSWORD", "")
	if err := svc.InitAdmin(); err == nil {
		t.Error("missing env should fail")
	}
}

func TestSendSMSCode(t *testing.T) {
	_, svc := setupService(t)
	if _, err := svc.SendSMSCode("13800000000", i18n.ZhCN, "bad-scene"); err == nil {
		t.Error("unsupported scene accepted")
	}
	code, err := svc.SendSMSCode("13800000000", i18n.ZhCN, sms.SceneLogin)
	if err != nil {
		t.Fatal(err)
	}
	if code == "" {
		t.Error("dev code empty")
	}
}

func TestLoginBySMS(t *testing.T) {
	db, svc := setupService(t)
	// auto-register path (fresh phone, single send avoids rate limit)
	phone := "13900000000"
	code, _ := svc.SendSMSCode(phone, i18n.ZhCN, sms.SceneLogin)
	pair, user, err := svc.LoginBySMS(phone, code)
	if err != nil {
		t.Fatal(err)
	}
	if user.ID == 0 {
		t.Error("sms auto-register failed")
	}
	if pair.AccessToken == "" {
		t.Error("no token")
	}
	// existing-user path: pre-create phone identity, then one send + login
	phone2 := "13900000001"
	u := mustCreateUser(t, db, "smsuser", "ValidPassword123!", 1)
	db.Create(&model.UserAuth{UserID: u.ID, IdentityType: "phone", Identifier: phone2})
	code2, _ := svc.SendSMSCode(phone2, i18n.ZhCN, sms.SceneLogin)
	if _, _, err := svc.LoginBySMS(phone2, code2); err != nil {
		t.Errorf("existing sms login failed: %v", err)
	}
	// wrong code
	if _, _, err := svc.LoginBySMS(phone, "000000"); err == nil {
		t.Error("wrong sms code accepted")
	}
}

func TestChannelsBindUnbind(t *testing.T) {
	_, svc := setupService(t)
	// Register gives the user a password identity so unbinding phone leaves one.
	user, err := svc.Register("erin", "ValidPassword123!", "Erin", "", "")
	if err != nil {
		t.Fatal(err)
	}
	phone := "13700000000"
	code, _ := svc.SendSMSCode(phone, i18n.ZhCN, sms.SceneBind)
	if err := svc.BindPhone(user.ID, phone, "000000"); err == nil {
		t.Error("wrong bind code accepted")
	}
	if err := svc.BindPhone(user.ID, phone, code); err != nil {
		t.Fatal(err)
	}
	chs, err := svc.GetUserChannels(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(chs) < 2 {
		t.Errorf("expected at least 2 channels, got %d", len(chs))
	}
	// duplicate bind
	if err := svc.BindPhone(user.ID, phone, code); err == nil {
		t.Error("duplicate phone bind accepted")
	}
	// unbind phone, leaves password channel
	if err := svc.UnbindChannel(user.ID, "phone"); err != nil {
		t.Fatal(err)
	}
	// cannot unbind last (password)
	if err := svc.UnbindChannel(user.ID, "password"); err == nil {
		t.Error("unbinding last channel accepted")
	}
}

func TestForgotResetPassword(t *testing.T) {
	db, svc := setupService(t)
	u := mustCreateUser(t, db, "frank", "ValidPassword123!", 1)
	phone := "13600000000"
	db.Create(&model.UserAuth{UserID: u.ID, IdentityType: "phone", Identifier: phone})
	code, err := svc.ForgotPassword(phone, i18n.ZhCN)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.ResetPassword(phone, "000000", "NewPassword123!"); err == nil {
		t.Error("wrong reset code accepted")
	}
	if err := svc.ResetPassword(phone, code, "NewPassword123!"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Login(u.Username, "NewPassword123!"); err != nil {
		t.Errorf("login with new password failed: %v", err)
	}
}

func TestGitHubFlowLocal(t *testing.T) {
	_, svc := setupService(t)
	url, state, err := svc.GetGitHubAuthURL()
	if err != nil {
		t.Fatal(err)
	}
	if url == "" || state == "" {
		t.Error("github auth url/state empty")
	}
	if err := svc.ConsumeOAuthState(""); err == nil {
		t.Error("empty state accepted")
	}
	if err := svc.ConsumeOAuthState(state); err != nil {
		t.Fatal(err)
	}
	if err := svc.ConsumeOAuthState(state); err == nil {
		t.Error("reused state accepted")
	}
}

func TestGitHubLoginNetwork(t *testing.T) {
	db, svc := setupService(t)
	// point GitHubOAuth at a test server returning token + user
	ts := newGitHubTestServer(t)
	defer ts.Close()
	GitHubOAuth = newTestGitHubOAuth(ts.URL)

	// pre-store state
	st := "state-xyz"
	db.Create(&model.OAuthState{ID: st, ExpiresAt: time.Now().Add(5 * time.Minute)})

	pair, user, err := svc.LoginByGitHub("code123")
	if err != nil {
		t.Fatal(err)
	}
	if user.ID == 0 || pair.AccessToken == "" {
		t.Error("github login failed")
	}
}

func TestGetFrontendURL(t *testing.T) {
	_, svc := setupService(t)
	if svc.GetFrontendURL() == "" {
		t.Error("frontend url empty")
	}
}
