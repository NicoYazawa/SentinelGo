package database

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// TraceIDKey 是用于在 context.Context 中存取 TraceID 的统一键。
// 中间件层和数据库层通过此键共享链路追踪 ID。
type traceIDKey struct{}

var TraceIDKey = traceIDKey{}

// ZapLogger 实现了 GORM 的 logger.Interface，将 GORM 日志输出到 slog 结构化日志。
// 自动记录 SQL 执行耗时、返回行数，并标记慢查询。
//
// 类型名与构造函数名沿用早期基于 Zap 的实现，实际输出走标准库 log/slog。
type ZapLogger struct {
	SlowThreshold time.Duration // 慢查询阈值，超过此时间的 SQL 会以 Warn 级别输出
}

// NewZapLogger 创建一个 slog 日志桥接器。
// slowThreshold 为慢查询判定时间，建议值 200ms。
func NewZapLogger(slowThreshold time.Duration) *ZapLogger {
	return &ZapLogger{SlowThreshold: slowThreshold}
}

// LogMode 满足 gormlogger.Interface，不改变日志级别。
func (z *ZapLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface { return z }

// Info 满足接口，静默丢弃（避免冗余的 INFO 级别 SQL 日志）。
func (z *ZapLogger) Info(ctx context.Context, s string, i ...interface{}) {}

// Warn 满足接口，静默丢弃。
func (z *ZapLogger) Warn(ctx context.Context, s string, i ...interface{}) {}

// Error 满足接口，静默丢弃（真正的错误由 Trace 方法统一输出）。
func (z *ZapLogger) Error(ctx context.Context, s string, i ...interface{}) {}

// Trace 是 GORM 日志的核心回调，在每次 SQL 执行后触发。
// 记录：执行耗时、返回/影响行数、SQL 语句、TraceID（从 context 提取）。
// 慢查询以 Warn 级别输出，SQL 错误以 Error 级别输出。
func (z *ZapLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	elapsed := time.Since(begin)
	sql, rows := fc()

	// 构造结构化日志字段
	attrs := []slog.Attr{
		slog.Duration("elapsed_ms", elapsed), // SQL 执行耗时
		slog.Int64("rows", rows),             // 扫描/影响行数
		slog.String("sql", sql),              // 实际执行的 SQL 语句
	}

	// 从 context 提取 TraceID，实现链路追踪
	if traceID, ok := ctx.Value(TraceIDKey).(string); ok && traceID != "" {
		attrs = append(attrs, slog.String("trace_id", traceID))
	}

	// GORM 将 ErrRecordNotFound 作为 error 传入，但这是正常业务逻辑，不记录 Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		slog.LogAttrs(ctx, slog.LevelError, "SQL 执行错误", append(attrs, slog.Any("error", err))...)
		return
	}

	// 超过阈值时发出慢查询警告
	if elapsed > z.SlowThreshold {
		slog.LogAttrs(ctx, slog.LevelWarn, fmt.Sprintf("慢查询检测 (超过 %v)", z.SlowThreshold), attrs...)
	}
}

// RegisterTracePlugin 为 GORM 注册 TraceID 传播插件。
// 在所有 SQL 操作（Query/Create/Update/Delete/Row/Raw）执行前，
// 自动将 context 中的 TraceID 注入到 GORM 内部 Context，确保每条 SQL 日志都能携带 TraceID。
func RegisterTracePlugin(db *gorm.DB) {
	// 覆盖所有 GORM 操作类型的 Before 回调
	db.Callback().Query().Before("gorm:query").Register("trace_inject", injectTraceID)
	db.Callback().Create().Before("gorm:create").Register("trace_inject", injectTraceID)
	db.Callback().Update().Before("gorm:update").Register("trace_inject", injectTraceID)
	db.Callback().Delete().Before("gorm:delete").Register("trace_inject", injectTraceID)
	db.Callback().Row().Before("gorm:row").Register("trace_inject", injectTraceID)
	db.Callback().Raw().Before("gorm:raw").Register("trace_inject", injectTraceID)
}

// injectTraceID 是 GORM 回调函数，在每次 SQL 执行前传播 TraceID。
// 优先从 GORM 内部 Context 中已有的 TraceIDKey 读取，
// 若不存在则尝试从通用的 "trace_id" 键读取并转换到统一键下。
func injectTraceID(db *gorm.DB) {
	// 已有 TraceIDKey 则跳过
	if db.Statement.Context != nil {
		if traceID, ok := db.Statement.Context.Value(TraceIDKey).(string); ok && traceID != "" {
			return
		}
	}
	// 尝试从中间件写入的 "trace_id" 键桥接到数据库统一键
	if db.Statement.Context != nil {
		if traceID, ok := db.Statement.Context.Value("trace_id").(string); ok && traceID != "" {
			db.Statement.Context = context.WithValue(db.Statement.Context, TraceIDKey, traceID)
		}
	}
}
