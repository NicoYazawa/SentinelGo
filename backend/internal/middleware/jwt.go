package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"sentinelgo/internal/repository"
	"sentinelgo/pkg/casbin"
	"sentinelgo/pkg/i18n"
	"sentinelgo/pkg/response"
	"sentinelgo/pkg/token"
)

// JWTAuth verifies a live database session and current enabled roles.
// JWT role claims and stale Casbin user mappings never grant access.
func JWTAuth(db *gorm.DB) gin.HandlerFunc {
	users := repository.NewUserRepository(db)
	sessions := repository.NewAuthSessionRepository(db)
	return func(c *gin.Context) {
		deny := func(code int, msg string) {
			c.AbortWithStatusJSON(http.StatusOK, response.Response{Code: code, Msg: msg, Data: gin.H{}})
		}
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			deny(response.ErrUnauthorized, "请先登录")
			return
		}
		claims, err := token.ParseToken(strings.TrimPrefix(header, "Bearer "))
		if err != nil || claims.TokenType != "access" {
			deny(response.ErrTokenExpired, "Token 已过期或无效")
			return
		}
		if err := sessions.Validate(claims.SessionID, claims.UserID); err != nil {
			deny(response.ErrUnauthorized, "登录会话已失效")
			return
		}
		user, err := users.FindForAuth(claims.UserID)
		if err != nil || user.Status != 1 {
			deny(response.ErrUnauthorized, "账户不可用，请重新登录")
			return
		}
		roles, err := users.GetEnabledRoles(claims.UserID)
		if err != nil {
			deny(response.ErrSystemError, "无法验证权限")
			return
		}
		names := make([]string, 0, len(roles))
		for _, role := range roles {
			names = append(names, role.Code)
		}
		c.Set("current_user_id", user.ID)
		c.Set("current_username", user.Username)
		c.Set("current_user_roles", names)
		c.Set("current_user_locale", user.Locale)
		c.Set("current_token_jti", claims.JTIShort)
		c.Set("current_session_id", claims.SessionID)

		// 语言跟随用户：当客户端未通过 X-Lang / Accept-Language 显式指定语言时，
		// 使用用户在库中的语言偏好（覆盖 I18n 中间件的默认值）。
		if user.Locale != "" {
			if l := i18n.Normalize(user.Locale); l != "" {
				if c.GetHeader(i18n.HeaderLang) == "" && c.GetHeader(i18n.HeaderAcceptLanguage) == "" {
					i18n.SetInContext(c, l)
					c.Header("Content-Language", string(l))
				}
			}
		}
		if user.PasswordChangeRequired && c.FullPath() != "/api/v1/auth/password/initialize" && c.FullPath() != "/api/v1/auth/logout" {
			deny(response.ErrPasswordChangeRequired, "请先设置正式密码")
			return
		}
		// Logout must remain available even after all roles are revoked.
		if c.FullPath() == "/api/v1/auth/logout" || c.FullPath() == "/api/v1/auth/password/initialize" {
			c.Next()
			return
		}
		if casbin.Enforcer == nil {
			deny(response.ErrSystemError, "权限服务不可用")
			return
		}
		allowed := false
		for _, role := range names {
			ok, err := casbin.Enforcer.Enforce(role, c.Request.URL.Path, c.Request.Method)
			if err != nil {
				deny(response.ErrSystemError, "无法验证权限")
				return
			}
			if ok {
				allowed = true
				break
			}
		}
		if !allowed {
			deny(response.ErrForbidden, "权限不足，拒绝访问")
			return
		}
		c.Next()
	}
}
