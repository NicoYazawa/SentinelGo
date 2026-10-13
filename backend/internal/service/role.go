package service

import (
	"errors"
	"fmt"

	"sentinelgo/internal/model"
	"sentinelgo/internal/repository"
	"sentinelgo/pkg/casbin"
)

// RoleService 是角色管理业务逻辑服务。
type RoleService struct {
	roleRepo *repository.RoleRepository
	menuRepo *repository.MenuRepository
}

// NewRoleService 创建角色服务（Wire Provider）。
func NewRoleService(roleRepo *repository.RoleRepository, menuRepo *repository.MenuRepository) *RoleService {
	return &RoleService{roleRepo: roleRepo, menuRepo: menuRepo}
}

// List 角色列表查询（支持关键词搜索）。
func (s *RoleService) List(keyword string) ([]model.Role, error) {
	return s.roleRepo.ListWithKeyword(keyword)
}

// GetByID 按 ID 查询角色。
func (s *RoleService) GetByID(id uint64) (*model.Role, error) {
	return s.roleRepo.FindByID(id)
}

// Create 创建新角色（默认状态启用）。
func (s *RoleService) Create(role *model.Role) error {
	role.Status = 1
	return s.roleRepo.Create(role)
}

// Update 更新角色信息（部分更新，仅更新有值的字段）。
func (s *RoleService) Update(id uint64, updates map[string]interface{}) error {
	role, err := s.roleRepo.FindByID(id)
	if err != nil {
		return err
	}
	if v, ok := updates["name"].(string); ok {
		role.Name = v
	}
	if v, ok := updates["desc"].(string); ok {
		role.Desc = v
	}
	if v, ok := updates["code"].(string); ok {
		role.Code = v
	}
	return s.roleRepo.Update(role)
}

// Delete 级联删除角色（含关联数据和 Casbin 策略）。
func (s *RoleService) Delete(id uint64) error {
	if err := s.roleRepo.DeleteCascade(id); err != nil {
		return err
	}
	return casbin.LoadPolicy()
}

// UpdateStatus 更新角色启用/禁用状态。
func (s *RoleService) UpdateStatus(id uint64, status int8) error {
	if status != 0 && status != 1 {
		return errors.New("状态必须为 0 或 1")
	}
	return s.roleRepo.UpdateStatus(id, status)
}

// AssignMenus 为角色分配菜单（替换式）。
func (s *RoleService) AssignMenus(roleID uint64, menuIDs []uint64) error {
	return s.roleRepo.AssignRoleMenus(roleID, menuIDs)
}

// GetMenus 获取角色拥有的菜单列表。
func (s *RoleService) GetMenus(roleID uint64) ([]model.Menu, error) {
	return s.roleRepo.GetRoleMenus(roleID)
}

// PermItem 是权限项定义，对应 Casbin 策略的 (obj, act) 两元组。
type PermItem struct {
	Path   string `json:"path"`   // API 路径
	Method string `json:"method"` // HTTP 方法
}

// AssignPermissions 为角色分配权限策略（先清除旧规则，再添加新规则）。
func (s *RoleService) AssignPermissions(roleID uint64, perms []PermItem) error {
	role, err := s.roleRepo.FindByID(roleID)
	if err != nil {
		return err
	}

	// 清除旧权限策略（p 规则中以角色 code 为 subject 的规则）
	if _, err := casbin.DeletePermissionsForUser(role.Code); err != nil {
		return fmt.Errorf("清除角色 %q 的旧权限策略失败: %w", role.Code, err)
	}

	// 添加新权限策略
	for _, p := range perms {
		if _, err := casbin.AddPolicy(role.Code, p.Path, p.Method); err != nil {
			return fmt.Errorf("添加角色 %q 的权限策略 [%s %s] 失败: %w", role.Code, p.Method, p.Path, err)
		}
	}

	// 重新加载，使内存策略与数据库保持一致
	return casbin.LoadPolicy()
}
