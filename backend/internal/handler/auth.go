// Package handler 提供 HTTP 控制层（Controller），处理请求解析、参数校验、响应返回。
//
// 设计原则：
//   - 仅做请求/响应转换，不包含业务逻辑
//   - 统一使用 response.ShouldBindJSON / response.Success / response.Fail
//   - 从 Gin Context 中提取当前用户信息（由 JWTAuth 中间件注入）
package handler

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"sentinelgo/internal/service"
	"sentinelgo/pkg/config"
	"sentinelgo/pkg/i18n"
	"sentinelgo/pkg/response"

	"github.com/gin-gonic/gin"
)

// smsText 是认证相关面向用户的中文/英文提示文案（按请求语言）。
var smsTextMap = map[i18n.Lang]map[string]string{
	i18n.ZhCN: {
		"needPhone":      "请输入手机号",
		"loginFirstBind": "请先登录再绑定手机号",
		"codeSent":       "验证码已发送",
	},
	i18n.EnUS: {
		"needPhone":      "Please enter your phone number",
		"loginFirstBind": "Please sign in before binding a phone number",
		"codeSent":       "Verification code sent",
	},
}

func smsText(lang i18n.Lang, key string) string {
	if m, ok := smsTextMap[lang]; ok {
		if v, ok := m[key]; ok {
			return v
		}
	}
	return smsTextMap[i18n.Default][key]
}

// AuthHandler 是认证相关的 HTTP 控制器。
type AuthHandler struct {
	authSvc *service.AuthService
}

// NewAuthHandler 创建认证 Handler（Wire Provider）。
func NewAuthHandler(authSvc *service.AuthService) *AuthHandler {
	return &AuthHandler{authSvc: authSvc}
}

// ---- 请求结构体定义 ----

type LoginReq struct {
	Username string `json:"username" binding:"required"` // 用户名
	Password string `json:"password" binding:"required"` // 密码
}

type RegisterReq struct {
	Username string `json:"username" binding:"required"` // 用户名
	Password string `json:"password" binding:"required"` // 密码
	Nickname string `json:"nickname"`                    // 昵称（可选）
	Email    string `json:"email"`                       // 邮箱（可选）
	Phone    string `json:"phone"`                       // 手机号（可选）
}

type RefreshReq struct {
	RefreshToken string `json:"refresh_token" binding:"required"` // Refresh Token
}

type SendSMSReq struct {
	Phone string `json:"phone" binding:"required"` // 手机号
	Scene string `json:"scene"`
}

type SMSLoginReq struct {
	Phone string `json:"phone" binding:"required"` // 手机号
	Code  string `json:"code" binding:"required"`  // 短信验证码
}

type BindPhoneReq struct {
	Phone string `json:"phone" binding:"required"` // 手机号
	Code  string `json:"code" binding:"required"`  // 短信验证码
}

type UnbindReq struct {
	IdentityType string `json:"identity_type" binding:"required"` // 要解绑的认证类型
}

type ForgotPwdReq struct {
	Phone string `json:"phone" binding:"required"` // 手机号
}

type ResetPwdReq struct {
	Phone       string `json:"phone" binding:"required"`        // 手机号
	Code        string `json:"code" binding:"required"`         // 短信验证码
	NewPassword string `json:"new_password" binding:"required"` // 新密码
}

// ---- 基础认证 ----

// Login 用户名+密码登录。
// POST /api/v1/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginReq
	if !response.ShouldBindJSON(c, &req) {
		return
	}

	pair, user, err := h.authSvc.Login(req.Username, req.Password)
	if err != nil {
		response.Fail(c, response.ErrUnauthorized, err.Error())
		return
	}

	// 写入操作人上下文，供审计中间件记录登录操作
	c.Set("current_user_id", user.ID)
	c.Set("current_username", user.Username)

	response.Success(c, gin.H{
		"user_id":                  user.ID,
		"username":                 user.Username,
		"access_token":             pair.AccessToken,
		"refresh_token":            pair.RefreshToken,
		"expires_in":               pair.ExpiresIn,
		"token_type":               "Bearer",
		"password_change_required": user.PasswordChangeRequired,
	})
}

// Register 新用户注册。
// POST /api/v1/auth/register
func (h *AuthHandler) Register(c *gin.Context) {
	// 检查系统是否开放注册开关
	if !config.GlobalManager.Get().SystemSwitches.EnableRegister {
		response.Fail(c, 403, "系统当前已暂停新用户注册，请联系管理员")
		return
	}

	var req RegisterReq
	if !response.ShouldBindJSON(c, &req) {
		return
	}

	user, err := h.authSvc.Register(req.Username, req.Password, req.Nickname, req.Email, req.Phone)
	if err != nil {
		response.Fail(c, response.ErrUserAlreadyExists, err.Error())
		return
	}

	response.Success(c, gin.H{
		"user_id":  user.ID,
		"username": user.Username,
	})
}

// Refresh 使用 Refresh Token 刷新 Access Token。
// POST /api/v1/auth/refresh
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req RefreshReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrParamInvalid, "请提供 Refresh Token")
		return
	}

	pair, err := h.authSvc.RefreshAccessToken(req.RefreshToken)
	if err != nil {
		response.Fail(c, response.ErrTokenExpired, err.Error())
		return
	}

	response.Success(c, gin.H{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"expires_in":    pair.ExpiresIn,
		"token_type":    "Bearer",
	})
}

// Logout 登出，将当前 Access Token 的 JTI 加入 Redis 黑名单。
// POST /api/v1/auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	sessionID, exists := c.Get("current_session_id")
	if !exists {
		response.Fail(c, response.ErrUnauthorized, "未登录")
		return
	}
	userID := c.GetUint64("current_user_id")
	if err := h.authSvc.Logout(userID, sessionID.(string)); err != nil {
		response.Fail(c, response.ErrSystemError, "退出登录失败，请重试")
		return
	}

	response.Success(c, gin.H{})
}

func (h *AuthHandler) InitializePassword(c *gin.Context) {
	var req struct {
		Current  string `json:"current_password" binding:"required"`
		Password string `json:"new_password" binding:"required"`
		Confirm  string `json:"confirm_password" binding:"required"`
	}
	if !response.ShouldBindJSON(c, &req) {
		return
	}
	if req.Password != req.Confirm {
		response.Fail(c, response.ErrParamInvalid, "两次输入的正式密码不一致")
		return
	}
	if err := h.authSvc.CompleteBootstrapPassword(c.GetUint64("current_user_id"), c.GetString("current_session_id"), req.Current, req.Password); err != nil {
		response.Fail(c, response.ErrParamInvalid, err.Error())
		return
	}
	response.Success(c, gin.H{})
}

func (h *AuthHandler) BootstrapStatus(c *gin.Context) {
	required, completed, err := h.authSvc.BootstrapStatus()
	if err != nil {
		response.Fail(c, response.ErrSystemError, "无法查询初始化状态")
		return
	}
	response.Success(c, gin.H{"password_change_required": required, "completed": completed})
}

// ---- 多渠道登录 ----

// SendSMS 发送短信验证码。
// POST /api/v1/auth/sms/send
func (h *AuthHandler) SendSMS(c *gin.Context) {
	lang := i18n.FromContext(c)
	var req SendSMSReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrParamInvalid, smsText(lang, "needPhone"))
		return
	}
	if req.Scene == "bind" {
		// Binding codes are issued only to a live authenticated account.
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") || h.authSvc.ValidateAccessToken(strings.TrimPrefix(header, "Bearer ")) != nil {
			response.Fail(c, response.ErrUnauthorized, smsText(lang, "loginFirstBind"))
			return
		}
	}
	code, err := h.authSvc.SendSMSCode(req.Phone, lang, req.Scene)
	if err != nil {
		response.Fail(c, response.ErrSystemError, err.Error())
		return
	}
	resp := gin.H{"msg": smsText(lang, "codeSent")}
	// 开发模式（dev 通道未真实下发短信）下，把验证码一并返回，方便前端直接展示，免去查日志。
	if config.GlobalManager.Get().App.Env == "dev" && config.GlobalManager.Get().SMS.Provider == "dev" {
		resp["dev_code"] = code
	}
	response.Success(c, resp)
}

// SMSLogin 短信验证码登录。
// POST /api/v1/auth/sms/login
func (h *AuthHandler) SMSLogin(c *gin.Context) {
	var req SMSLoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrParamInvalid, "请输入手机号和验证码")
		return
	}

	pair, user, err := h.authSvc.LoginBySMS(req.Phone, req.Code)
	if err != nil {
		// 注意：此处不能用 ErrUnauthorized(20000)，否则前端拦截器会误判为 token 过期而触发刷新死循环。
		// 验证码错误属于普通业务校验失败，使用 ErrParamInvalid(11000)，前端会直接 ElMessage.error(res.msg)。
		response.Fail(c, response.ErrParamInvalid, err.Error())
		return
	}

	response.Success(c, gin.H{
		"user_id":                  user.ID,
		"username":                 user.Username,
		"access_token":             pair.AccessToken,
		"refresh_token":            pair.RefreshToken,
		"expires_in":               pair.ExpiresIn,
		"token_type":               "Bearer",
		"password_change_required": user.PasswordChangeRequired,
	})
}

// GitHubOAuth 跳转到 GitHub 授权页面。
// GET /api/v1/auth/oauth/github
func (h *AuthHandler) GitHubOAuth(c *gin.Context) {
	authURL, state, err := h.authSvc.GetGitHubAuthURL()
	if err != nil {
		response.Fail(c, response.ErrSystemError, err.Error())
		return
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("sentinel_oauth_state", state, 300, "/api/v1/auth/oauth/github/callback", "", c.Request.TLS != nil, true)
	c.Redirect(302, authURL)
}

// GitHubCallback 处理 GitHub OAuth 回调，登录或自动注册后重定向到前端。
// GET /api/v1/auth/oauth/github/callback?code=xxx
func (h *AuthHandler) GitHubCallback(c *gin.Context) {
	state := c.Query("state")
	cookie, err := c.Cookie("sentinel_oauth_state")
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("sentinel_oauth_state", "", -1, "/api/v1/auth/oauth/github/callback", "", c.Request.TLS != nil, true)
	if err != nil || state == "" || subtle.ConstantTimeCompare([]byte(state), []byte(cookie)) != 1 {
		response.Fail(c, response.ErrParamInvalid, "OAuth state 校验失败")
		return
	}
	if err := h.authSvc.ConsumeOAuthState(state); err != nil {
		response.Fail(c, response.ErrParamInvalid, err.Error())
		return
	}
	code := c.Query("code")
	if code == "" {
		response.Fail(c, response.ErrParamInvalid, "缺少授权码")
		return
	}

	pair, user, err := h.authSvc.LoginByGitHub(code)
	if err != nil {
		response.Fail(c, response.ErrUnauthorized, err.Error())
		return
	}

	// 将 Token 作为 URL Fragment 参数传递到前端（不经过服务端日志）
	frontendURL := h.authSvc.GetFrontendURL()
	redirectURL := fmt.Sprintf("%s/auth/oauth/callback#access_token=%s&refresh_token=%s&expires_in=%d&user_id=%d&username=%s&password_change_required=%t",
		frontendURL, pair.AccessToken, pair.RefreshToken, pair.ExpiresIn, user.ID, url.QueryEscape(user.Username), user.PasswordChangeRequired)
	c.Redirect(302, redirectURL)
}

// ---- 账户绑定 ----

// GetChannels 获取当前用户的所有认证渠道。
// GET /api/v1/auth/channels
func (h *AuthHandler) GetChannels(c *gin.Context) {
	userID, _ := c.Get("current_user_id")
	channels, err := h.authSvc.GetUserChannels(userID.(uint64))
	if err != nil {
		response.Fail(c, response.ErrSystemError, err.Error())
		return
	}
	response.Success(c, channels)
}

// BindPhone 绑定手机号到当前账户。
// POST /api/v1/auth/bind/phone
func (h *AuthHandler) BindPhone(c *gin.Context) {
	var req BindPhoneReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrParamInvalid, "请输入手机号和验证码")
		return
	}
	userID, _ := c.Get("current_user_id")
	if err := h.authSvc.BindPhone(userID.(uint64), req.Phone, req.Code); err != nil {
		response.Fail(c, response.ErrParamInvalid, err.Error())
		return
	}
	response.Success(c, gin.H{})
}

// Unbind 解绑指定的认证渠道。
// POST /api/v1/auth/unbind
func (h *AuthHandler) Unbind(c *gin.Context) {
	var req UnbindReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrParamInvalid, "请指定要解绑的登录方式")
		return
	}
	userID, _ := c.Get("current_user_id")
	if err := h.authSvc.UnbindChannel(userID.(uint64), req.IdentityType); err != nil {
		response.Fail(c, response.ErrParamInvalid, err.Error())
		return
	}
	response.Success(c, gin.H{})
}

// ---- 找回密码 ----

// ForgotPassword 向已注册手机号发送找回密码验证码。
// POST /api/v1/auth/password/forgot
func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	lang := i18n.FromContext(c)
	var req ForgotPwdReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrParamInvalid, smsText(lang, "needPhone"))
		return
	}
	code, err := h.authSvc.ForgotPassword(req.Phone, lang)
	if err != nil {
		response.Fail(c, response.ErrParamInvalid, err.Error())
		return
	}
	resp := gin.H{"msg": smsText(lang, "codeSent")}
	// 开发模式（dev 通道未真实下发短信）下，把验证码一并返回，方便前端直接展示，免去查日志。
	if config.GlobalManager.Get().App.Env == "dev" && config.GlobalManager.Get().SMS.Provider == "dev" {
		resp["dev_code"] = code
	}
	response.Success(c, resp)
}

// ResetPassword 验证短信验证码后重置密码。
// POST /api/v1/auth/password/reset
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req ResetPwdReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrParamInvalid, "请填写完整信息")
		return
	}
	if err := h.authSvc.ResetPassword(req.Phone, req.Code, req.NewPassword); err != nil {
		response.Fail(c, response.ErrParamInvalid, err.Error())
		return
	}
	response.Success(c, gin.H{"msg": "密码重置成功，请重新登录"})
}
