package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Cors 是跨域资源共享（CORS）中间件，允许前端跨域请求。
//
// 配置说明：
//   - 允许来源：反射请求的 Origin 头（不限制域名，Credentials 模式下不可用 "*"）
//   - 允许方法：GET, POST, PUT, DELETE, OPTIONS, UPDATE, PATCH
//   - 允许请求头：Authorization（JWT）、X-Trace-Id（链路追踪）、Content-Type 等
//   - 暴露响应头：X-Trace-Id（供前端获取 TraceID）
//   - 允许携带 Cookie（Access-Control-Allow-Credentials: true）
//
// 对 OPTIONS 预检请求直接返回 204 No Content，不进入后续 Handler 链。
func Cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, UPDATE, PATCH")
			c.Header("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Authorization, X-Trace-Id")
			c.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, X-Trace-Id")
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		// OPTIONS 预检请求直接返回，不再执行后续中间件和 Handler
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
