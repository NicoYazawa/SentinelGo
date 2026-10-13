package repository

import (
	"sentinelgo/internal/model"

	"gorm.io/gorm"
)

// PermissionRepository 是接口权限定义的数据访问层。
type PermissionRepository struct {
	*BaseRepository[model.Permission]
	db *gorm.DB
}

// NewPermissionRepository 创建权限 Repository（Wire Provider）。
func NewPermissionRepository(db *gorm.DB) *PermissionRepository {
	return &PermissionRepository{
		BaseRepository: NewBaseRepository[model.Permission](db),
		db:             db,
	}
}

// FindByPathAndMethod 按路径+方法精确查询权限定义（组合唯一）。
func (r *PermissionRepository) FindByPathAndMethod(path, method string) (*model.Permission, error) {
	var perm model.Permission
	err := r.db.Where("path = ? AND method = ?", path, method).First(&perm).Error
	return &perm, err
}

// FindByStatus 按启用状态查询权限列表。
func (r *PermissionRepository) FindByStatus(status int8) ([]model.Permission, error) {
	var perms []model.Permission
	err := r.db.Where("status = ?", status).Find(&perms).Error
	return perms, err
}

// ListWithFilter 多条件分页查询权限列表。
//
// 参数：
//   - keyword: 模糊匹配 path 和 description
//   - method: HTTP 方法过滤（为空则不过滤）
//   - status: nil 表示不过滤状态，否则按指定状态过滤
func (r *PermissionRepository) ListWithFilter(keyword, method string, status *int8, page, pageSize int) ([]model.Permission, int64, error) {
	var perms []model.Permission
	var total int64

	q := r.db.Model(&model.Permission{})
	if keyword != "" {
		q = q.Where("path LIKE ? OR description LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if method != "" {
		q = q.Where("method = ?", method)
	}
	if status != nil {
		q = q.Where("status = ?", *status)
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := q.Offset(offset).Limit(pageSize).Order("id ASC").Find(&perms).Error
	return perms, total, err
}
