package repository

import (
	"context"
	"time"

	"sentinelgo/internal/model"

	"gorm.io/gorm"
)

// OperationLogRepository 是操作日志的数据访问层。
type OperationLogRepository struct {
	db *gorm.DB
}

// NewOperationLogRepository 创建操作日志 Repository（Wire Provider）。
func NewOperationLogRepository(db *gorm.DB) *OperationLogRepository {
	return &OperationLogRepository{db: db}
}

// Create 写入一条操作审计记录。
func (r *OperationLogRepository) Create(ctx context.Context, log *model.OperationLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// OpLogQuery 操作日志查询条件。
type OpLogQuery struct {
	Username string // 操作人（模糊匹配）
	Module   string // 业务模块（精确）
	Method   string // HTTP 方法（精确）
	Status   int    // 0=全部 1=成功 2=失败
	Start    string // 起始时间（YYYY-MM-DD HH:MM:SS）
	End      string // 结束时间（YYYY-MM-DD HH:MM:SS）
	Page     int    // 页码，从 1 开始
	PageSize int    // 每页条数
}

// List 分页 + 多条件查询操作日志，返回数据列表与总数。
func (r *OperationLogRepository) List(ctx context.Context, q OpLogQuery) ([]model.OperationLog, int64, error) {
	db := r.db.WithContext(ctx).Model(&model.OperationLog{})

	if q.Username != "" {
		db = db.Where("username LIKE ?", "%"+q.Username+"%")
	}
	if q.Module != "" {
		db = db.Where("module = ?", q.Module)
	}
	if q.Method != "" {
		db = db.Where("method = ?", q.Method)
	}
	switch q.Status {
	case 1:
		db = db.Where("status = ?", 1)
	case 2:
		db = db.Where("status = ?", 0)
	}
	if q.Start != "" {
		if t, err := time.ParseInLocation("2006-01-02 15:04:05", q.Start, time.Local); err == nil {
			db = db.Where("created_at >= ?", t)
		}
	}
	if q.End != "" {
		if t, err := time.ParseInLocation("2006-01-02 15:04:05", q.End, time.Local); err == nil {
			db = db.Where("created_at <= ?", t)
		}
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page := q.Page
	if page <= 0 {
		page = 1
	}
	pageSize := q.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}

	var list []model.OperationLog
	if err := db.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}
