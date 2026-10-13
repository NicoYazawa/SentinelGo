package repository

import (
	"sentinelgo/internal/model"

	"gorm.io/gorm"
)

// MenuRepository 是菜单数据访问层。
type MenuRepository struct {
	*BaseRepository[model.Menu]
	db *gorm.DB
}

// NewMenuRepository 创建菜单 Repository（Wire Provider）。
func NewMenuRepository(db *gorm.DB) *MenuRepository {
	return &MenuRepository{
		BaseRepository: NewBaseRepository[model.Menu](db),
		db:             db,
	}
}

// ListTree 查询所有菜单（按 sort 排序），供 BuildMenuTree 构建树形结构。
func (r *MenuRepository) ListTree() ([]model.Menu, error) {
	var menus []model.Menu
	err := r.db.Order("sort ASC").Find(&menus).Error
	return menus, err
}

// ListByType 按类型过滤菜单列表（1=目录 2=菜单 3=按钮）。
func (r *MenuRepository) ListByType(menuType int) ([]model.Menu, error) {
	var menus []model.Menu
	err := r.db.Where("type = ?", menuType).Order("sort ASC").Find(&menus).Error
	return menus, err
}

// FindByPath 按前端路由路径查询菜单。
func (r *MenuRepository) FindByPath(path string) (*model.Menu, error) {
	var menu model.Menu
	err := r.db.Where("path = ?", path).First(&menu).Error
	return &menu, err
}

// UpdateStatus 更新菜单启用/禁用状态。
func (r *MenuRepository) UpdateStatus(id uint64, status int8) error {
	return r.db.Model(&model.Menu{}).Where("id = ?", id).Update("status", status).Error
}

// GetUserMenus 查询指定用户拥有的菜单（通过 user_roles → role_menus 两表关联）。
// 仅返回状态为启用的菜单，按 sort 排序。
//
// SQL 逻辑：
//
//	SELECT DISTINCT m.* FROM sys_menu m
//	  JOIN role_menus rm ON m.id = rm.menu_id
//	  JOIN user_roles ur ON rm.role_id = ur.role_id
//	  WHERE ur.user_id = ? AND m.status = 1
//	  ORDER BY m.sort ASC
func (r *MenuRepository) GetUserMenus(userID uint64) ([]model.Menu, error) {
	var menus []model.Menu
	err := r.db.Raw(`
		SELECT DISTINCT m.* FROM sys_menu m
		INNER JOIN role_menus rm ON m.id = rm.menu_id
		INNER JOIN user_roles ur ON rm.role_id = ur.role_id
		WHERE ur.user_id = ? AND m.status = 1
		ORDER BY m.sort ASC
	`, userID).Scan(&menus).Error
	return menus, err
}
