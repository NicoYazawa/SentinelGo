package repository

import (
	"sentinelgo/internal/model"

	"gorm.io/gorm"
)

// RoleRepository 是角色数据访问层。
type RoleRepository struct {
	*BaseRepository[model.Role]
	db *gorm.DB
}

// NewRoleRepository 创建角色 Repository（Wire Provider）。
func NewRoleRepository(db *gorm.DB) *RoleRepository {
	return &RoleRepository{
		BaseRepository: NewBaseRepository[model.Role](db),
		db:             db,
	}
}

// FindByCode 按角色编码精确查询。
func (r *RoleRepository) FindByCode(code string) (*model.Role, error) {
	var role model.Role
	err := r.db.Where("code = ?", code).First(&role).Error
	return &role, err
}

// ListByStatus 按状态过滤角色列表（1=启用 0=禁用）。
func (r *RoleRepository) ListByStatus(status int8) ([]model.Role, error) {
	var roles []model.Role
	err := r.db.Where("status = ?", status).Find(&roles).Error
	return roles, err
}

// UpdateStatus 更新角色启用/禁用状态。
func (r *RoleRepository) UpdateStatus(id uint64, status int8) error {
	return r.db.Model(&model.Role{}).Where("id = ?", id).Update("status", status).Error
}

// GetRoleMenus 获取角色拥有的所有菜单（预加载 Menus 关联）。
func (r *RoleRepository) GetRoleMenus(roleID uint64) ([]model.Menu, error) {
	var role model.Role
	if err := r.db.Preload("Menus").First(&role, roleID).Error; err != nil {
		return nil, err
	}
	return role.Menus, nil
}

// AssignRoleMenus 替换角色的菜单分配（先清空再设置）。
// 使用 GORM Association Replace，自动管理 role_menus 中间表。
func (r *RoleRepository) AssignRoleMenus(roleID uint64, menuIDs []uint64) error {
	var role model.Role
	role.ID = roleID

	var menus []model.Menu
	for _, mid := range menuIDs {
		menus = append(menus, model.Menu{BaseModel: model.BaseModel{ID: mid}})
	}

	return r.db.Model(&role).Association("Menus").Replace(menus)
}

// ListWithKeyword 角色列表查询（分页+关键词）。
func (r *RoleRepository) ListWithKeyword(keyword string) ([]model.Role, error) {
	var roles []model.Role
	q := r.db.Model(&model.Role{})
	if keyword != "" {
		q = q.Where("name LIKE ? OR code LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	err := q.Order("id ASC").Find(&roles).Error
	return roles, err
}

// DeleteCascade 级联删除角色，同时清理关联数据。
// 使用 GORM 事务保证原子性：
//  1. 删除 user_roles 中的角色关联
//  2. 删除 role_menus 中的菜单关联
//  3. 删除 casbin_rule 中的权限规则
//  4. 删除角色本身
func (r *RoleRepository) DeleteCascade(roleID uint64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var role model.Role
		if err := tx.First(&role, roleID).Error; err != nil {
			return err
		}
		if err := tx.Where("role_id = ?", roleID).Delete(&model.UserRole{}).Error; err != nil {
			return err
		}
		if err := tx.Where("role_id = ?", roleID).Delete(&model.RoleMenu{}).Error; err != nil {
			return err
		}
		if err := tx.Where("(ptype = ? AND v0 = ?) OR (ptype = ? AND v1 = ?)", "p", role.Code, "g", role.Code).Delete(&model.CasbinRule{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Role{}, roleID).Error
	})
}
