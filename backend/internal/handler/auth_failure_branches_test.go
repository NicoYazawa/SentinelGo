package handler

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"sentinelgo/internal/model"
	"sentinelgo/pkg/config"
	"sentinelgo/pkg/i18n"
	"sentinelgo/pkg/response"
)

func TestAuthMissingFields(t *testing.T) {
	db, s, _ := setupHandler(t)
	h := NewAuthHandler(s)
	cases := []struct {
		name, msg string
		fn        gin.HandlerFunc
	}{{"login", "", h.Login}, {"register", "", h.Register}, {"refresh", "请提供 Refresh Token", h.Refresh}, {"initialize", "", h.InitializePassword}, {"sms send", "请输入手机号", h.SendSMS}, {"sms login", "请输入手机号和验证码", h.SMSLogin}, {"bind", "请输入手机号和验证码", h.BindPhone}, {"unbind", "请指定要解绑的登录方式", h.Unbind}, {"forgot", "请输入手机号", h.ForgotPassword}, {"reset", "请填写完整信息", h.ResetPassword}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, w := hctx(t, "POST", "/auth", map[string]string{}, 0, "")
			tc.fn(c)
			got := decodeH(t, w)
			if got.Code != response.ErrParamInvalid || got.Msg == "" || (tc.msg != "" && got.Msg != tc.msg) {
				t.Fatalf("unexpected validation response: %+v", got)
			}
			if _, exists := c.Get("current_user_id"); exists {
				t.Fatal("invalid input authenticated user")
			}
		})
	}
	var count int64
	if err := db.Model(&model.User{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("invalid authentication requests created user")
	}
}
func TestAuthBusinessRejections(t *testing.T) {
	db, s, _ := setupHandler(t)
	h := NewAuthHandler(s)
	u := mustCreateUser(t, db, "unchanged", "ValidPassword123!", 1)
	cases := []struct {
		name, body, msg string
		code            int
		fn              gin.HandlerFunc
	}{
		{"invalid refresh", `{"refresh_token":"invalid"}`, "", response.ErrTokenExpired, h.Refresh},
		{"invalid sms scene", `{"phone":"13811112222","scene":"unknown"}`, "不支持的验证码场景", response.ErrSystemError, h.SendSMS},
		{"binding without session", `{"phone":"13811112222","scene":"bind"}`, "请先登录再绑定手机号", response.ErrUnauthorized, h.SendSMS},
		{"binding wrong code", `{"phone":"13811112222","code":"bad"}`, "验证码错误或已过期", response.ErrParamInvalid, h.BindPhone},
		{"unbind last channel", `{"identity_type":"password"}`, "至少保留一种登录方式", response.ErrParamInvalid, h.Unbind},
		{"forgot unknown phone", `{"phone":"13811112222"}`, "该手机号未注册", response.ErrParamInvalid, h.ForgotPassword},
		{"reset wrong code", `{"phone":"13811112222","code":"bad","new_password":"OtherPassword123!"}`, "验证码错误或已过期", response.ErrParamInvalid, h.ResetPassword},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, w := hctxRaw(t, "POST", "/auth", []byte(tc.body), u.ID, "")
			tc.fn(c)
			got := decodeH(t, w)
			if got.Code != tc.code || got.Msg == "" || (tc.msg != "" && got.Msg != tc.msg) {
				t.Fatalf("unexpected rejection: %+v", got)
			}
		})
	}
	var persisted model.User
	if err := db.First(&persisted, u.ID).Error; err != nil {
		t.Fatal(err)
	}
	if persisted.Password != u.Password || persisted.Phone != "" {
		t.Fatal("rejected auth operation changed account")
	}
	cfg := *config.GlobalManager.Get()
	cfg.SystemSwitches.EnableRegister = false
	config.GlobalManager.Update(&cfg)
	c, w := hctx(t, "POST", "/auth/register", map[string]string{"username": "blocked", "password": "ValidPassword123!"}, 0, "")
	h.Register(c)
	if got := decodeH(t, w); got.Code != 403 || !strings.Contains(got.Msg, "暂停") {
		t.Fatalf("registration gate: %+v", got)
	}
}
func TestAuthStorageFailures(t *testing.T) {
	db, s, _ := setupHandler(t)
	h := NewAuthHandler(s)
	sqlDB, _ := db.DB()
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, msg string
		fn        gin.HandlerFunc
	}{{"bootstrap", "无法查询初始化状态", h.BootstrapStatus}, {"logout", "退出登录失败，请重试", h.Logout}, {"channels", "closed", h.GetChannels}, {"oauth", "closed", h.GitHubOAuth}} {
		t.Run(tc.name, func(t *testing.T) {
			c, w := hctx(t, "POST", "/auth", nil, 1, "session")
			tc.fn(c)
			got := decodeH(t, w)
			if got.Code != response.ErrSystemError || !strings.Contains(got.Msg, tc.msg) {
				t.Fatalf("unexpected failure %+v", got)
			}
			if w.Header().Get("Location") != "" {
				t.Fatal("failed OAuth redirected")
			}
		})
	}
}
func TestOAuthCallbackRejections(t *testing.T) {
	db, s, _ := setupHandler(t)
	h := NewAuthHandler(s)
	for _, tc := range []struct {
		name, state, cookie, code, msg string
		save                           bool
	}{{"missing cookie", "state", "", "code", "OAuth state 校验失败", false}, {"mismatch", "state", "other", "code", "OAuth state 校验失败", false}, {"unknown state", "state", "state", "code", "", false}, {"missing code", "valid", "valid", "", "缺少授权码", true}} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.save {
				if err := db.Create(&model.OAuthState{ID: tc.state, ExpiresAt: time.Now().Add(time.Minute)}).Error; err != nil {
					t.Fatal(err)
				}
			}
			c, w := hctx(t, "GET", "/callback?state="+tc.state+"&code="+tc.code, nil, 0, "")
			if tc.cookie != "" {
				c.Request.AddCookie(&http.Cookie{Name: "sentinel_oauth_state", Value: tc.cookie})
			}
			h.GitHubCallback(c)
			got := decodeH(t, w)
			if got.Code != response.ErrParamInvalid || got.Msg == "" || (tc.msg != "" && got.Msg != tc.msg) {
				t.Fatalf("unexpected OAuth rejection %+v", got)
			}
			if w.Header().Get("Location") != "" {
				t.Fatal("invalid OAuth redirected")
			}
			cookies := w.Result().Cookies()
			if len(cookies) != 1 || cookies[0].MaxAge >= 0 {
				t.Fatal("state cookie not cleared")
			}
		})
	}
}
func TestSMSLanguageFallback(t *testing.T) {
	if got := smsText(i18n.Lang("unknown"), "needPhone"); got != "请输入手机号" {
		t.Fatalf("fallback %q", got)
	}
	if got := smsText(i18n.EnUS, "unknown"); got != "" {
		t.Fatalf("unknown key %q", got)
	}
}
