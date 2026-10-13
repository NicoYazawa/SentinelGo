package service

import (
	"fmt"

	"gorm.io/gorm"

	"sentinelgo/internal/model"
	"sentinelgo/internal/repository"
	"sentinelgo/pkg/utils"
)

// adminRoleID 是内置管理员角色的固定 ID。
const adminRoleID uint64 = 1

// MenuService 是菜单管理业务逻辑服务。
type MenuService struct {
	menuRepo *repository.MenuRepository
	roleRepo *repository.RoleRepository
	db       *gorm.DB
}

// NewMenuService 创建菜单服务（Wire Provider）。
func NewMenuService(menuRepo *repository.MenuRepository, roleRepo *repository.RoleRepository, db *gorm.DB) *MenuService {
	return &MenuService{menuRepo: menuRepo, roleRepo: roleRepo, db: db}
}

// ListTree 返回菜单树形结构（调用 utils.BuildMenuTree 将扁平列表转为嵌套树）。
func (s *MenuService) ListTree() ([]*utils.MenuTreeResp, error) {
	menus, err := s.menuRepo.ListTree()
	if err != nil {
		return nil, err
	}
	return utils.BuildMenuTree(menus), nil
}

// GetByID 按 ID 查询单个菜单。
func (s *MenuService) GetByID(id uint64) (*model.Menu, error) {
	return s.menuRepo.FindByID(id)
}

// Create 创建菜单。
func (s *MenuService) Create(menu *model.Menu) error {
	return s.menuRepo.Create(menu)
}

// Update 更新菜单（部分更新，仅更新有值的字段，Hidden 直接覆盖）。
func (s *MenuService) Update(id uint64, menu *model.Menu) error {
	existing, err := s.menuRepo.FindByID(id)
	if err != nil {
		return err
	}
	if menu.Name != "" {
		existing.Name = menu.Name
	}
	if menu.Path != "" {
		existing.Path = menu.Path
	}
	if menu.Component != "" {
		existing.Component = menu.Component
	}
	if menu.Icon != "" {
		existing.Icon = menu.Icon
	}
	if menu.Sort != 0 {
		existing.Sort = menu.Sort
	}
	if menu.Type != 0 {
		existing.Type = menu.Type
	}
	existing.Hidden = menu.Hidden
	existing.Perms = menu.Perms
	return s.menuRepo.Update(existing)
}

// Delete 删除菜单。
func (s *MenuService) Delete(id uint64) error {
	return s.menuRepo.Delete(id)
}

// UpdateStatus 更新菜单启用/禁用状态。
func (s *MenuService) UpdateStatus(id uint64, status int8) error {
	return s.menuRepo.UpdateStatus(id, status)
}

// GetUserMenus 获取指定用户可访问的菜单树（通过角色-菜单关联）。
// 查询结果为空时兜底返回全部菜单，避免新用户看到空侧边栏。
func (s *MenuService) GetUserMenus(userID uint64) ([]*utils.MenuTreeResp, error) {
	menus, err := s.menuRepo.GetUserMenus(userID)
	if err != nil {
		return nil, err
	}
	if len(menus) == 0 {
		// 兜底：用户无角色分配时返回所有菜单
		menus, err = s.menuRepo.ListTree()
		if err != nil {
			return nil, fmt.Errorf("查询全部菜单失败: %w", err)
		}
	}
	return utils.BuildMenuTree(menus), nil
}

// InitDefaultMenus 初始化系统默认菜单结构（使用固定 ID + FirstOrCreate 确保幂等）。
// 菜单结构：
//
//	首页 (/dashboard)
//	系统管理 (/system)
//	  ├── 用户管理 (/system/users)
//	  ├── 角色管理 (/system/roles)
//	  ├── 菜单管理 (/system/menus)
//	  ├── 权限管理 (/system/permissions)
//	  └── 系统监控 (/system/monitor)
//
// 初始化完成后自动将所有菜单分配给 admin 角色（ID=1）。
func (s *MenuService) InitDefaultMenus() error {
	menus := []model.Menu{
		{BaseModel: model.BaseModel{ID: 1}, Name: "首页", Path: "/dashboard", Component: "dashboard/index", Icon: "HomeFilled", Sort: 1, Type: 2, Status: 1},
		{BaseModel: model.BaseModel{ID: 2}, Name: "系统管理", Path: "/system", Icon: "Setting", Sort: 2, Type: 1, Status: 1},
		{BaseModel: model.BaseModel{ID: 3}, ParentID: 2, Name: "用户管理", Path: "/system/users", Component: "system/user/index", Icon: "User", Sort: 1, Type: 2, Status: 1},
		{BaseModel: model.BaseModel{ID: 4}, ParentID: 2, Name: "角色管理", Path: "/system/roles", Component: "system/role/index", Icon: "UserFilled", Sort: 2, Type: 2, Status: 1},
		{BaseModel: model.BaseModel{ID: 5}, ParentID: 2, Name: "菜单管理", Path: "/system/menus", Component: "system/menu/index", Icon: "Menu", Sort: 3, Type: 2, Status: 1},
		{BaseModel: model.BaseModel{ID: 6}, ParentID: 2, Name: "权限管理", Path: "/system/permissions", Component: "system/permission/index", Icon: "Key", Sort: 4, Type: 2, Status: 1},
		{BaseModel: model.BaseModel{ID: 7}, ParentID: 2, Name: "系统监控", Path: "/system/monitor", Component: "system/monitor/index", Icon: "Monitor", Sort: 5, Type: 2, Status: 1},
		{BaseModel: model.BaseModel{ID: 8}, ParentID: 2, Name: "操作日志", Path: "/system/audit", Component: "system/audit/index", Icon: "Document", Sort: 6, Type: 2, Status: 1},
	}

	// FirstOrCreate 确保重复启动不会创建重复菜单
	allMenuIDs := make([]uint64, 0, len(menus))
	for _, m := range menus {
		if err := s.db.Where("id = ?", m.ID).FirstOrCreate(&m).Error; err != nil {
			return fmt.Errorf("初始化菜单 %q 失败: %w", m.Name, err)
		}
		allMenuIDs = append(allMenuIDs, m.ID)
	}

	// 分配所有菜单给 admin 角色
	if err := s.roleRepo.AssignRoleMenus(adminRoleID, allMenuIDs); err != nil {
		return fmt.Errorf("将默认菜单分配给 admin 角色失败: %w", err)
	}
	return nil
}
