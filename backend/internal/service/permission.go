package service

import (
	"sentinelgo/internal/model"
	"sentinelgo/internal/repository"

	"gorm.io/gorm"
)

// PermissionService 是权限定义管理业务逻辑服务。
type PermissionService struct {
	permRepo *repository.PermissionRepository
	db       *gorm.DB
}

// NewPermissionService 创建权限服务（Wire Provider）。
func NewPermissionService(permRepo *repository.PermissionRepository, db *gorm.DB) *PermissionService {
	return &PermissionService{permRepo: permRepo, db: db}
}

// PermissionListReq 权限列表查询请求参数。
type PermissionListReq struct {
	Keyword  string `form:"keyword"`   // 模糊搜索（匹配 path 和 description）
	Method   string `form:"method"`    // HTTP 方法过滤
	Status   *int8  `form:"status"`    // 状态过滤（nil 表示不过滤）
	Page     int    `form:"page"`      // 页码
	PageSize int    `form:"page_size"` // 每页条数
}

// List 多条件分页查询权限列表。
func (s *PermissionService) List(req PermissionListReq) ([]model.Permission, int64, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	return s.permRepo.ListWithFilter(req.Keyword, req.Method, req.Status, req.Page, req.PageSize)
}

// GetByID 按 ID 查询权限定义。
func (s *PermissionService) GetByID(id uint64) (*model.Permission, error) {
	return s.permRepo.FindByID(id)
}

// Create 创建权限定义（默认状态启用）。
func (s *PermissionService) Create(perm *model.Permission) error {
	perm.Status = 1
	return s.permRepo.Create(perm)
}

// Update 更新权限定义（部分更新）。
func (s *PermissionService) Update(id uint64, perm *model.Permission) error {
	existing, err := s.permRepo.FindByID(id)
	if err != nil {
		return err
	}
	if perm.Path != "" {
		existing.Path = perm.Path
	}
	if perm.Method != "" {
		existing.Method = perm.Method
	}
	if perm.Description != "" {
		existing.Description = perm.Description
	}
	existing.Status = perm.Status
	return s.permRepo.Update(existing)
}

// Delete 删除权限定义。
func (s *PermissionService) Delete(id uint64) error {
	return s.permRepo.Delete(id)
}

// InitDefaultPermissions 初始化默认权限定义（仅在表为空时执行，保证幂等）。
func (s *PermissionService) InitDefaultPermissions() error {
	var count int64
	s.db.Model(&model.Permission{}).Count(&count)
	if count > 0 {
		return nil // 已有数据，跳过初始化
	}

	// 默认权限定义：各资源的 CRUD 操作
	perms := []model.Permission{
		{Path: "/api/v1/users", Method: "GET", Description: "查询用户列表"},
		{Path: "/api/v1/users", Method: "POST", Description: "创建用户"},
		{Path: "/api/v1/users", Method: "PUT", Description: "更新用户"},
		{Path: "/api/v1/users", Method: "DELETE", Description: "删除用户"},
		{Path: "/api/v1/roles", Method: "GET", Description: "查询角色列表"},
		{Path: "/api/v1/roles", Method: "POST", Description: "创建角色"},
		{Path: "/api/v1/roles", Method: "PUT", Description: "更新角色"},
		{Path: "/api/v1/roles", Method: "DELETE", Description: "删除角色"},
		{Path: "/api/v1/menus", Method: "GET", Description: "查询菜单列表"},
		{Path: "/api/v1/menus", Method: "POST", Description: "创建菜单"},
		{Path: "/api/v1/menus", Method: "PUT", Description: "更新菜单"},
		{Path: "/api/v1/menus", Method: "DELETE", Description: "删除菜单"},
		{Path: "/api/v1/permissions", Method: "GET", Description: "查询权限列表"},
		{Path: "/api/v1/permissions", Method: "POST", Description: "创建权限"},
		{Path: "/api/v1/permissions", Method: "PUT", Description: "更新权限"},
		{Path: "/api/v1/permissions", Method: "DELETE", Description: "删除权限"},
	}

	for _, p := range perms {
		s.permRepo.Create(&p)
	}
	return nil
}
