package handler

import (
	"sentinelgo/internal/service"
	"sentinelgo/pkg/response"

	"github.com/gin-gonic/gin"
)

// DashboardHandler 是首页仪表盘的 HTTP 控制器，提供聚合统计接口。
type DashboardHandler struct {
	svc *service.DashboardService
}

// NewDashboardHandler 创建仪表盘 Handler（Wire Provider）。
func NewDashboardHandler(svc *service.DashboardService) *DashboardHandler {
	return &DashboardHandler{svc: svc}
}

// Stats 返回首页仪表盘所需的全部聚合指标（KPI + 安全审计看板数据）。
// GET /api/v1/dashboard/stats
//
// 数据来源：
//   - 用户/角色/权限/菜单总数：对应物理表 Count
//   - 今日维度统计：created_at >= 今日 00:00:00
//   - 登录统计：来自审计日志（module=认证、path 以 /login 结尾、按 status 区分成败）
//   - 最近操作/异常：sys_operation_log 倒序取最近 N 条
func (h *DashboardHandler) Stats(c *gin.Context) {
	data, err := h.svc.Stats()
	if err != nil {
		response.Fail(c, response.ErrSystemError, err.Error())
		return
	}
	response.Success(c, data)
}
