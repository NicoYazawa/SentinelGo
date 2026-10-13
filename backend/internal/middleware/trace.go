// Package middleware 提供 Gin 框架的 HTTP 中间件集合。
//
// 中间件执行顺序（按 NewRouter 中 Use 的注册顺序）：
//
//	Recovery → Trace → CORS → Logger → Metrics → RateLimit → JWTAuth → CasbinCheck
//
// 关键设计：
//   - TraceID 贯穿整个请求生命周期：HTTP Header → Gin Context → request context → GORM callback → Zap log
//   - 统一使用 database.TraceIDKey 作为 context 键，确保中间件层与数据库层的 TraceID 互通
package middleware

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"sentinelgo/pkg/database"
)

// TraceIDKey 是 Gin Context 中存储 TraceID 的字符串键（用于 c.Get/c.Set）。
const TraceIDKey = "trace_id"

// Trace 是链路追踪中间件，为每个请求生成或继承 TraceID。
//
// 优先级：
//  1. 从请求头 X-Trace-Id 继承（支持跨服务追踪）
//  2. 不存在则生成新的 UUID v4
//
// TraceID 会同时注入：
//   - Gin Context（通过 c.Set）
//   - Response Header（X-Trace-Id）
//   - Request Context（通过 context.WithValue + database.TraceIDKey）
//
// 注入到 Request Context 的 TraceID 会被 GORM 的 RegisterTracePlugin 读取，
// 从而在每条 SQL 日志中自动携带 TraceID，实现全链路追踪。
func Trace() gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := c.GetHeader("X-Trace-Id")
		if traceID == "" {
			traceID = uuid.New().String() // 生成新的 UUID v4
		}

		// 注入 Gin Context（后续中间件和 Handler 通过 c.Get 获取）
		c.Set(TraceIDKey, traceID)
		// 注入 Response Header（客户端可据此关联请求）
		c.Header("X-Trace-Id", traceID)

		// 注入 Request Context（GORM 插件通过 database.TraceIDKey 读取）
		ctx := context.WithValue(c.Request.Context(), database.TraceIDKey, traceID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// GetTraceID 从标准 context.Context 中提取 TraceID。
// 用于非 HTTP 上下文（如后台任务）中获取链路追踪 ID。
func GetTraceID(ctx context.Context) string {
	if v, ok := ctx.Value(database.TraceIDKey).(string); ok {
		return v
	}
	return ""
}
