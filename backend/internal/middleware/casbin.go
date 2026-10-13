package middleware

import (
	"log/slog"
	"net/http"

	"github.com/casbin/casbin/v3"
	"github.com/gin-gonic/gin"

	"sentinelgo/pkg/response"
)

// Enforcer 是 Casbin 同步执行器的本地引用，由 InitCasbin 注入。
var Enforcer *casbin.SyncedEnforcer

// InitCasbin 注入 Casbin Enforcer 实例给中间件使用。
// 在 main.go 中 Casbin 初始化完成后调用。
func InitCasbin(enf *casbin.SyncedEnforcer) {
	Enforcer = enf
}

// CasbinCheck 是基于角色的权限鉴权中间件（独立于 JWTAuth 中的鉴权逻辑）。
//
// 与 JWTAuth 中鉴权的区别：
//   - JWTAuth 中基于 userID 做鉴权（主体为用户）
//   - CasbinCheck 中基于角色列表做鉴权（主体为角色）
//
// 使用方式：可在特定路由组上单独注册，或与 JWTAuth 配合使用。
// 遍历当前用户的所有角色，任一角色有权限即放行。
func CasbinCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从 Gin Context 取 JWTAuth 注入的角色列表
		rolesInterface, exists := c.Get("current_user_roles")
		if !exists {
			c.AbortWithStatusJSON(http.StatusOK, response.Response{
				Code:    response.ErrForbidden,
				Msg:     "未获取到用户角色，拒绝访问",
				TraceID: "",
				Data:    gin.H{},
			})
			return
		}
		roles := rolesInterface.([]string)

		// 构造 Casbin 鉴权参数
		obj := c.Request.URL.Path // 请求路径
		act := c.Request.Method   // HTTP 方法

		// 遍历角色，任一匹配即可放行
		hasAccess := false
		for _, role := range roles {
			ok, err := Enforcer.Enforce(role, obj, act)
			if err != nil {
				slog.Error("Casbin 鉴权失败", "role", role, "obj", obj, "act", act, "err", err)
				continue
			}
			if ok {
				hasAccess = true
				break
			}
		}

		if !hasAccess {
			c.AbortWithStatusJSON(http.StatusOK, response.Response{
				Code:    response.ErrForbidden,
				Msg:     "权限不足，拒绝访问",
				TraceID: "",
				Data:    gin.H{},
			})
			return
		}

		c.Next()
	}
}
