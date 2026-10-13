package service

import (
	"context"

	"sentinelgo/internal/model"
	"sentinelgo/internal/repository"
)

// OperationLogService 是操作日志业务逻辑层。
type OperationLogService struct {
	repo *repository.OperationLogRepository
}

// NewOperationLogService 创建操作日志服务（Wire Provider）。
func NewOperationLogService(repo *repository.OperationLogRepository) *OperationLogService {
	return &OperationLogService{repo: repo}
}

// List 分页查询操作日志。
func (s *OperationLogService) List(ctx context.Context, q repository.OpLogQuery) ([]model.OperationLog, int64, error) {
	return s.repo.List(ctx, q)
}
