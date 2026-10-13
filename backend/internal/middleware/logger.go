package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// Logger 是结构化请求日志中间件，记录每个 HTTP 请求的关键信息。
//
// 记录字段：
//   - trace_id: 链路追踪 ID（关联 GORM SQL 日志 + 业务日志）
//   - status: HTTP 响应状态码
//   - method + path + query: 请求方法和路径
//   - ip: 客户端 IP
//   - cost: 请求处理耗时
//   - user-agent: 客户端标识
//
// 日志级别统一为 Info，异常请求通过 status 过滤查询。
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// 先执行后续中间件和 Handler
		c.Next()

		// 请求结束后记录日志
		cost := time.Since(start)
		traceID, _ := c.Get(TraceIDKey)

		slog.Info("HTTP Request",
			"trace_id", traceID,
			"status", c.Writer.Status(),
			"method", c.Request.Method,
			"path", path,
			"query", query,
			"ip", c.ClientIP(),
			"cost", cost,
			"user-agent", c.Request.UserAgent(),
		)
	}
}
