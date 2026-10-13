package handler

import (
	"strconv"

	"sentinelgo/internal/repository"
	"sentinelgo/internal/service"
	"sentinelgo/pkg/response"

	"github.com/gin-gonic/gin"
)

// OperationLogHandler 是操作日志的 HTTP 控制器。
type OperationLogHandler struct {
	svc *service.OperationLogService
}

// NewOperationLogHandler 创建操作日志 Handler（Wire Provider）。
func NewOperationLogHandler(svc *service.OperationLogService) *OperationLogHandler {
	return &OperationLogHandler{svc: svc}
}

// List 查询操作日志列表（支持操作人/模块/方法/结果/时间范围筛选 + 分页）。
// GET /api/v1/audit/logs
func (h *OperationLogHandler) List(c *gin.Context) {
	q := buildOpLogQuery(c)
	list, total, err := h.svc.List(c.Request.Context(), q)
	if err != nil {
		response.Fail(c, response.ErrSystemError, err.Error())
		return
	}
	response.Success(c, gin.H{"list": list, "total": total})
}

// buildOpLogQuery 从请求 Query 解析操作日志查询条件。
func buildOpLogQuery(c *gin.Context) repository.OpLogQuery {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	status, _ := strconv.Atoi(c.DefaultQuery("status", "0"))

	return repository.OpLogQuery{
		Username: c.Query("username"),
		Module:   c.Query("module"),
		Method:   c.Query("method"),
		Status:   status,
		Start:    c.Query("start"),
		End:      c.Query("end"),
		Page:     page,
		PageSize: pageSize,
	}
}
