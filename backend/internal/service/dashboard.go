package service

import (
	"sentinelgo/internal/model"
	"sentinelgo/internal/repository"
)

// DashboardData 仪表盘聚合数据，直接作为接口响应体返回。
// json tag 采用 snake_case，与前端约定一致。
type DashboardData struct {
	TotalUsers        int64                `json:"total_users"`         // 系统总用户数
	TodayNewUsers     int64                `json:"today_new_users"`     // 今日新增用户数
	TotalRoles        int64                `json:"total_roles"`         // 角色总数
	TotalPermissions  int64                `json:"total_permissions"`   // 权限定义总数
	TotalMenus        int64                `json:"total_menus"`         // 菜单总数
	TodayOperations   int64                `json:"today_operations"`    // 今日操作总数
	TodayFailedOps    int64                `json:"today_failed_ops"`    // 今日失败操作数
	TodayLogins       int64                `json:"today_logins"`        // 今日登录成功次数
	TodayFailedLogins int64                `json:"today_failed_logins"` // 今日登录失败次数
	RecentOperations  []model.OperationLog `json:"recent_operations"`   // 最近操作动态（10 条）
	RecentFailedOps   []model.OperationLog `json:"recent_failed_ops"`   // 最近异常操作（8 条）
}

// DashboardService 仪表盘业务逻辑层，编排各 Repository 聚合查询。
type DashboardService struct {
	repo *repository.DashboardRepository
}

// NewDashboardService 创建仪表盘服务（Wire Provider）。
func NewDashboardService(repo *repository.DashboardRepository) *DashboardService {
	return &DashboardService{repo: repo}
}

// Stats 聚合仪表盘所需的全部指标。
// 登录统计依赖审计日志中 module=认证 且 path 以 /login 结尾 的记录。
func (s *DashboardService) Stats() (*DashboardData, error) {
	d := &DashboardData{}
	var err error

	if d.TotalUsers, err = s.repo.CountAll("sys_user"); err != nil {
		return nil, err
	}
	if d.TodayNewUsers, err = s.repo.CountToday("sys_user", ""); err != nil {
		return nil, err
	}
	if d.TotalRoles, err = s.repo.CountAll("sys_role"); err != nil {
		return nil, err
	}
	if d.TotalPermissions, err = s.repo.CountAll("sys_permission"); err != nil {
		return nil, err
	}
	if d.TotalMenus, err = s.repo.CountAll("sys_menu"); err != nil {
		return nil, err
	}
	if d.TodayOperations, err = s.repo.CountToday("sys_operation_log", ""); err != nil {
		return nil, err
	}
	// 失败操作：失败操作的总数（含登录失败）。它是一个总类，失败登录（today_failed_logins）是其子类。
	// 注意：这里统计所有 status=0 的记录，不做互斥排除，概览前端会对「失败操作」做展开以查看子类明细。
	if d.TodayFailedOps, err = s.repo.CountToday("sys_operation_log", "status = ?", 0); err != nil {
		return nil, err
	}
	if d.TodayLogins, err = s.repo.CountToday("sys_operation_log", "module = ? AND path LIKE ?", "认证", "%/login"); err != nil {
		return nil, err
	}
	if d.TodayFailedLogins, err = s.repo.CountToday("sys_operation_log", "module = ? AND path LIKE ? AND status = ?", "认证", "%/login", 0); err != nil {
		return nil, err
	}
	if d.RecentOperations, err = s.repo.RecentOperations(10, false, false); err != nil {
		return nil, err
	}
	// 最近异常操作：包含登录失败等全部失败记录，作为「失败操作」展开后的明细来源。
	if d.RecentFailedOps, err = s.repo.RecentOperations(8, true, false); err != nil {
		return nil, err
	}
	return d, nil
}
