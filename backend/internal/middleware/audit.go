package middleware

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"sentinelgo/internal/model"
	"sentinelgo/internal/repository"
)

// responseCapture 包裹 gin.ResponseWriter，在转发的同时缓冲响应体，
// 用于事后解析业务 code（本项目 HTTP 始终 200，业务状态由 code 字段区分）。
type responseCapture struct {
	gin.ResponseWriter
	body []byte
}

// Write 转发给底层 Writer 并缓存一份响应体。
func (w *responseCapture) Write(b []byte) (int, error) {
	w.body = append(w.body, b...)
	return w.ResponseWriter.Write(b)
}

// bizResponse 仅解析响应体中的业务 code 与 msg。
type bizResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// NewAuditMiddleware 创建操作审计日志中间件。
//
// 行为：
//   - 仅记录写操作（POST/PUT/DELETE/PATCH），读请求（GET）直接放行，避免日志爆炸。
//   - 从 JWTAuth 注入的上下文读取操作人（current_user_id / current_username）。
//   - 通过响应体中的业务 code 判断操作成败（0 成功，非 0 失败）。
//   - 异步写入 sys_operation_log，不阻塞主请求。
func NewAuditMiddleware(repo *repository.OperationLogRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 只审计写操作
		if c.Request.Method == http.MethodGet {
			c.Next()
			return
		}

		writer := &responseCapture{ResponseWriter: c.Writer}
		c.Writer = writer

		c.Next()

		// 解析业务结果。响应体可能为空或非 JSON（如 204 No Content），
		// 此时按业务失败处理并记录日志，但不应中断审计流程。
		var biz bizResponse
		if err := json.Unmarshal(writer.body, &biz); err != nil {
			slog.Warn("审计中间件解析响应体失败",
				"path", c.Request.URL.Path,
				"method", c.Request.Method,
				"body_size", len(writer.body),
				"err", err)
		}
		success := biz.Code == 0

		// 读取操作人（JWTAuth 注入）
		var userID uint64
		if v, ok := c.Get("current_user_id"); ok {
			if id, ok := v.(uint64); ok {
				userID = id
			}
		}
		var username string
		if v, ok := c.Get("current_username"); ok {
			if name, ok := v.(string); ok {
				username = name
			}
		}
		var traceID string
		if v, ok := c.Get("trace_id"); ok {
			if t, ok := v.(string); ok {
				traceID = t
			}
		}

		module, verb := parseModuleAndVerb(c.Request.URL.Path, c.Request.Method)
		status := 1
		errMsg := ""
		if !success {
			status = 0
			errMsg = biz.Msg
		}

		log := &model.OperationLog{
			UserID:    userID,
			Username:  username,
			Module:    module,
			Operation: module + "-" + verb,
			Method:    c.Request.Method,
			Path:      c.Request.URL.Path,
			IP:        c.ClientIP(),
			Status:    status,
			ErrorMsg:  errMsg,
			UserAgent: c.Request.UserAgent(),
			TraceID:   traceID,
		}

		// 异步落库，避免阻塞请求链路。
		// 请求 context 在响应返回后即被取消，此处必须使用独立的 Background context。
		go func(entry *model.OperationLog) {
			if err := repo.Create(context.Background(), entry); err != nil {
				slog.Error("写入操作审计日志失败",
					"path", entry.Path,
					"method", entry.Method,
					"user_id", entry.UserID,
					"err", err)
			}
		}(log)
	}
}

// parseModuleAndVerb 根据请求路径与方法推导业务模块与动作描述。
func parseModuleAndVerb(path, method string) (module, verb string) {
	switch {
	case strings.Contains(path, "/users"):
		module = "用户"
	case strings.Contains(path, "/roles"):
		module = "角色"
	case strings.Contains(path, "/menus"):
		module = "菜单"
	case strings.Contains(path, "/permissions"):
		module = "权限"
	case strings.Contains(path, "/auth"):
		module = "认证"
	case strings.Contains(path, "/user/profile"), strings.Contains(path, "/user/avatar"):
		module = "个人中心"
	default:
		module = "其他"
	}

	switch method {
	case http.MethodPost:
		verb = "创建"
	case http.MethodPut, http.MethodPatch:
		verb = "更新"
	case http.MethodDelete:
		verb = "删除"
	default:
		verb = "操作"
	}

	if module == "认证" && strings.HasSuffix(path, "/login") {
		verb = "登录"
	}
	return module, verb
}
