package repository

import (
	"time"

	"sentinelgo/internal/model"

	"gorm.io/gorm"
)

// DashboardRepository 仪表盘统计聚合查询层。
// 集中提供「总数统计」与「按日统计」两类查询，避免在前端多次请求。
type DashboardRepository struct {
	db *gorm.DB
}

// NewDashboardRepository 创建仪表盘 Repository（Wire Provider）。
func NewDashboardRepository(db *gorm.DB) *DashboardRepository {
	return &DashboardRepository{db: db}
}

// CountAll 统计指定物理表的总记录数。
// table 为物理表名（如 sys_user），复用 BaseModel 约定。
func (r *DashboardRepository) CountAll(table string) (int64, error) {
	var n int64
	err := r.db.Table(table).Count(&n).Error
	return n, err
}

// CountToday 统计指定表「当天」（created_at >= 今日 00:00:00）的记录数。
// extraWhere / args 可附加额外过滤条件（如 status = ?、module = ?）。
func (r *DashboardRepository) CountToday(table, extraWhere string, args ...interface{}) (int64, error) {
	q := r.db.Table(table).Where("created_at >= ?", time.Now().Format("2006-01-02"))
	if extraWhere != "" {
		q = q.Where(extraWhere, args...)
	}
	var n int64
	err := q.Count(&n).Error
	return n, err
}

// RecentOperations 按 ID 倒序取最近的操作审计记录。
// failedOnly=true 时仅返回失败（status=0）的记录，用于「异常操作」看板。
// excludeLogin=true 时排除登录类失败（module=认证 且 path 以 /login 结尾），
// 与仪表盘「失败操作」计数口径保持一致，避免与「失败登录」指标重叠。
func (r *DashboardRepository) RecentOperations(limit int, failedOnly, excludeLogin bool) ([]model.OperationLog, error) {
	var logs []model.OperationLog
	q := r.db.Model(&model.OperationLog{}).Order("id DESC").Limit(limit)
	if failedOnly {
		q = q.Where("status = ?", 0)
	}
	if excludeLogin {
		q = q.Where("NOT (module = ? AND path LIKE ?)", "认证", "%/login")
	}
	err := q.Find(&logs).Error
	return logs, err
}
