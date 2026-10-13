package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"sentinelgo/internal/model"
	"sentinelgo/internal/service"
	"sentinelgo/pkg/i18n"
	"sentinelgo/pkg/sms"
)

type hresp struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

func decodeH(t *testing.T, w *httptest.ResponseRecorder) hresp {
	t.Helper()
	var r hresp
	if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil {
		t.Fatalf("decode response: %v body=%s", err, w.Body.String())
	}
	return r
}

func TestAuthLogin(t *testing.T) {
	db, authSvc, _ := setupHandler(t)
	h := NewAuthHandler(authSvc)
	u := mustCreateUser(t, db, "alice", "ValidPassword123!", 1)
	mustAssignRole(t, db, u.ID, "user")

	c, w := hctx(t, http.MethodPost, "/api/v1/auth/login", map[string]string{"username": "alice", "password": "ValidPassword123!"}, 0, "")
	h.Login(c)
	r := decodeH(t, w)
	if r.Code != 0 {
		t.Fatalf("login failed: %+v", r)
	}
	var data struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(r.Data, &data); err != nil {
		t.Fatal(err)
	}
	if data.AccessToken == "" || data.RefreshToken == "" {
		t.Error("tokens missing")
	}
}

func TestAuthLoginBadPassword(t *testing.T) {
	db, authSvc, _ := setupHandler(t)
	h := NewAuthHandler(authSvc)
	mustCreateUser(t, db, "bob", "ValidPassword123!", 1)
	c, w := hctx(t, http.MethodPost, "/api/v1/auth/login", map[string]string{"username": "bob", "password": "wrong"}, 0, "")
	h.Login(c)
	if decodeH(t, w).Code == 0 {
		t.Error("bad password should fail")
	}
}

func TestAuthRegister(t *testing.T) {
	_, authSvc, _ := setupHandler(t)
	h := NewAuthHandler(authSvc)
	c, w := hctx(t, http.MethodPost, "/api/v1/auth/register", map[string]string{"username": "newbie", "password": "ValidPassword123!"}, 0, "")
	h.Register(c)
	if decodeH(t, w).Code != 0 {
		t.Error("register failed")
	}
	// duplicate
	c2, w2 := hctx(t, http.MethodPost, "/api/v1/auth/register", map[string]string{"username": "newbie", "password": "ValidPassword123!"}, 0, "")
	h.Register(c2)
	if decodeH(t, w2).Code == 0 {
		t.Error("duplicate register accepted")
	}
}

func TestAuthRefreshAndLogout(t *testing.T) {
	db, authSvc, _ := setupHandler(t)
	h := NewAuthHandler(authSvc)
	u := mustCreateUser(t, db, "carol", "ValidPassword123!", 1)
	mustAssignRole(t, db, u.ID, "user")
	lc, lw := hctx(t, http.MethodPost, "/api/v1/auth/login", map[string]string{"username": "carol", "password": "ValidPassword123!"}, 0, "")
	h.Login(lc)
	var loginData struct {
		RefreshToken string `json:"refresh_token"`
	}
	_ = json.Unmarshal(decodeH(t, lw).Data, &loginData)

	rc, rw := hctx(t, http.MethodPost, "/api/v1/auth/refresh", map[string]string{"refresh_token": loginData.RefreshToken}, 0, "")
	h.Refresh(rc)
	if decodeH(t, rw).Code != 0 {
		t.Error("refresh failed")
	}

	// logout without session id -> 未登录
	lo, loW := hctx(t, http.MethodPost, "/api/v1/auth/logout", nil, u.ID, "")
	h.Logout(lo)
	if decodeH(t, loW).Code == 0 {
		t.Error("logout without session should fail")
	}
	// logout with valid session
	db.Create(&model.AuthSession{ID: "sess-carol", UserID: u.ID, RefreshJTI: "j", ExpiresAt: time.Now().Add(time.Hour)})
	lo2, loW2 := hctx(t, http.MethodPost, "/api/v1/auth/logout", nil, u.ID, "sess-carol")
	h.Logout(lo2)
	if decodeH(t, loW2).Code != 0 {
		t.Errorf("logout failed: %+v", decodeH(t, loW2))
	}
}

func TestAuthInitializePassword(t *testing.T) {
	db, authSvc, _ := setupHandler(t)
	h := NewAuthHandler(authSvc)
	temp := "TemporaryPassword123!"
	u := mustCreateUser(t, db, "dave", temp, 1)
	u.PasswordChangeRequired = true
	db.Model(&model.User{}).Where("id = ?", u.ID).Update("password_change_required", true)
	db.Create(&model.AuthSession{ID: "sess-dave", UserID: u.ID, RefreshJTI: "j", ExpiresAt: time.Now().Add(time.Hour)})

	// wrong current
	c, w := hctx(t, http.MethodPost, "/api/v1/auth/password/initialize", map[string]string{"current_password": "x", "new_password": "FormalPassword456!", "confirm_password": "FormalPassword456!"}, u.ID, "sess-dave")
	h.InitializePassword(c)
	if decodeH(t, w).Code == 0 {
		t.Error("wrong current accepted")
	}
	// success
	c2, w2 := hctx(t, http.MethodPost, "/api/v1/auth/password/initialize", map[string]string{"current_password": temp, "new_password": "FormalPassword456!", "confirm_password": "FormalPassword456!"}, u.ID, "sess-dave")
	h.InitializePassword(c2)
	if decodeH(t, w2).Code != 0 {
		t.Errorf("init password failed: %+v", decodeH(t, w2))
	}
	// mismatch confirm
	c3, w3 := hctx(t, http.MethodPost, "/api/v1/auth/password/initialize", map[string]string{"current_password": temp, "new_password": "FormalPassword457!", "confirm_password": "Other457!"}, u.ID, "sess-dave")
	h.InitializePassword(c3)
	if decodeH(t, w3).Code == 0 {
		t.Error("mismatched confirm accepted")
	}
}

func TestAuthBootstrapStatus(t *testing.T) {
	_, authSvc, _ := setupHandler(t)
	h := NewAuthHandler(authSvc)
	c, w := hctx(t, http.MethodGet, "/api/v1/auth/bootstrap/status", nil, 0, "")
	h.BootstrapStatus(c)
	if decodeH(t, w).Code != 0 {
		t.Error("bootstrap status failed")
	}
}

func TestAuthSendSMSAndLogin(t *testing.T) {
	_, authSvc, _ := setupHandler(t)
	h := NewAuthHandler(authSvc)
	phone := "13900000000"
	sc, sw := hctx(t, http.MethodPost, "/api/v1/auth/sms/send", map[string]string{"phone": phone, "scene": "login"}, 0, "")
	h.SendSMS(sc)
	r := decodeH(t, sw)
	if r.Code != 0 {
		t.Fatalf("send sms failed: %+v", r)
	}
	var sent struct {
		DevCode string `json:"dev_code"`
	}
	_ = json.Unmarshal(r.Data, &sent)
	if sent.DevCode == "" {
		t.Fatal("dev code missing")
	}
	lc, lw := hctx(t, http.MethodPost, "/api/v1/auth/sms/login", map[string]string{"phone": phone, "code": sent.DevCode}, 0, "")
	h.SMSLogin(lc)
	if decodeH(t, lw).Code != 0 {
		t.Error("sms login failed")
	}
	// wrong code
	wc, ww := hctx(t, http.MethodPost, "/api/v1/auth/sms/login", map[string]string{"phone": phone, "code": "000000"}, 0, "")
	h.SMSLogin(wc)
	if decodeH(t, ww).Code == 0 {
		t.Error("wrong sms code accepted")
	}
}

func TestAuthChannelsBindUnbind(t *testing.T) {
	_, authSvc, _ := setupHandler(t)
	h := NewAuthHandler(authSvc)
	user, err := authSvc.Register("erin", "ValidPassword123!", "Erin", "", "")
	if err != nil {
		t.Fatal(err)
	}
	phone := "13700000000"
	// generate a bind code at the service level (no auth header needed)
	code, err := authSvc.SendSMSCode(phone, i18n.ZhCN, sms.SceneBind)
	if err != nil {
		t.Fatal(err)
	}
	bc, bw := hctx(t, http.MethodPost, "/api/v1/auth/bind/phone", map[string]string{"phone": phone, "code": code}, user.ID, "")
	h.BindPhone(bc)
	if decodeH(t, bw).Code != 0 {
		t.Errorf("bind phone failed: %+v", decodeH(t, bw))
	}
	gc, gw := hctx(t, http.MethodGet, "/api/v1/auth/channels", nil, user.ID, "")
	h.GetChannels(gc)
	if decodeH(t, gw).Code != 0 {
		t.Error("get channels failed")
	}
	uc, uw := hctx(t, http.MethodPost, "/api/v1/auth/unbind", map[string]string{"identity_type": "phone"}, user.ID, "")
	h.Unbind(uc)
	if decodeH(t, uw).Code != 0 {
		t.Error("unbind failed")
	}
}

func TestAuthForgotReset(t *testing.T) {
	db, authSvc, _ := setupHandler(t)
	h := NewAuthHandler(authSvc)
	u := mustCreateUser(t, db, "frank", "ValidPassword123!", 1)
	phone := "13600000000"
	db.Create(&model.UserAuth{UserID: u.ID, IdentityType: "phone", Identifier: phone})

	fc, fw := hctx(t, http.MethodPost, "/api/v1/auth/password/forgot", map[string]string{"phone": phone}, 0, "")
	h.ForgotPassword(fc)
	code := extractDevCode(t, fw)
	if code == "" {
		t.Fatal("forgot code missing")
	}
	rc, rw := hctx(t, http.MethodPost, "/api/v1/auth/password/reset", map[string]string{"phone": phone, "code": code, "new_password": "NewPassword123!"}, 0, "")
	h.ResetPassword(rc)
	if decodeH(t, rw).Code != 0 {
		t.Errorf("reset failed: %+v", decodeH(t, rw))
	}
}

func TestAuthGitHubOAuth(t *testing.T) {
	_, authSvc, _ := setupHandler(t)
	h := NewAuthHandler(authSvc)
	c, w := hctx(t, http.MethodGet, "/api/v1/auth/oauth/github", nil, 0, "")
	h.GitHubOAuth(c)
	if w.Code != http.StatusFound && w.Code != http.StatusTemporaryRedirect {
		t.Errorf("expected redirect, got %d", w.Code)
	}
	if w.Header().Get("Location") == "" {
		t.Error("no location header")
	}
	if w.Result().Cookies() == nil {
		t.Error("no state cookie")
	}
}

func TestAuthGitHubCallback(t *testing.T) {
	db, authSvc, _ := setupHandler(t)
	h := NewAuthHandler(authSvc)
	ts := newGitHubTestServer(t)
	defer ts.Close()
	service.GitHubOAuth = newTestGitHubOAuth(ts.URL)
	db.Create(&model.OAuthState{ID: "st", ExpiresAt: time.Now().Add(time.Minute)})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/github/callback?state=st&code=code123", nil)
	req.AddCookie(&http.Cookie{Name: "sentinel_oauth_state", Value: "st"})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	h.GitHubCallback(c)
	if w.Code != http.StatusFound {
		t.Errorf("expected redirect, got %d body=%s", w.Code, w.Body.String())
	}
}

func extractDevCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var r struct {
		Data struct {
			DevCode string `json:"dev_code"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil {
		return ""
	}
	return r.Data.DevCode
}
