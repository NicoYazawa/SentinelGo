package handler

import (
	"strconv"

	"sentinelgo/internal/model"
	"sentinelgo/internal/service"
	"sentinelgo/pkg/response"

	"github.com/gin-gonic/gin"
)

// RoleHandler 是角色管理的 HTTP 控制器，提供角色的 CRUD、菜单分配、权限策略分配。
type RoleHandler struct {
	roleSvc *service.RoleService
}

// NewRoleHandler 创建角色 Handler（Wire Provider）。
func NewRoleHandler(roleSvc *service.RoleService) *RoleHandler {
	return &RoleHandler{roleSvc: roleSvc}
}

// List 查询角色列表，支持关键词模糊搜索。
// GET /api/v1/roles?keyword=xxx
func (h *RoleHandler) List(c *gin.Context) {
	keyword := c.Query("keyword")
	roles, err := h.roleSvc.List(keyword)
	if err != nil {
		response.Fail(c, response.ErrSystemError, err.Error())
		return
	}
	response.Success(c, roles)
}

// Get 按 ID 查询单个角色详情。
// GET /api/v1/roles/:id
func (h *RoleHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	role, err := h.roleSvc.GetByID(id)
	if err != nil {
		response.Fail(c, response.ErrSystemError, "角色不存在")
		return
	}
	response.Success(c, role)
}

// Create 创建新角色（默认状态启用）。
// POST /api/v1/roles
func (h *RoleHandler) Create(c *gin.Context) {
	var role model.Role
	if err := c.ShouldBindJSON(&role); err != nil {
		response.Fail(c, response.ErrParamInvalid, "参数错误")
		return
	}
	if err := h.roleSvc.Create(&role); err != nil {
		response.Fail(c, response.ErrSystemError, err.Error())
		return
	}
	response.Success(c, role)
}

// Update 更新角色信息（部分更新，支持 name/desc/code 字段）。
// PUT /api/v1/roles/:id
func (h *RoleHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	// 使用 map 接收以支持部分字段更新
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		response.Fail(c, response.ErrParamInvalid, "参数错误")
		return
	}
	if err := h.roleSvc.Update(id, updates); err != nil {
		response.Fail(c, response.ErrSystemError, err.Error())
		return
	}
	response.Success(c, gin.H{})
}

// Delete 级联删除角色，同时清理关联的 user_roles、role_menus、Casbin 策略。
// DELETE /api/v1/roles/:id
func (h *RoleHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.roleSvc.Delete(id); err != nil {
		response.Fail(c, response.ErrSystemError, err.Error())
		return
	}
	response.Success(c, gin.H{})
}

// UpdateStatus 更新角色启用/禁用状态。
// PUT /api/v1/roles/:id/status
func (h *RoleHandler) UpdateStatus(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		Status *int8 `json:"status" binding:"required"` // 1=启用, 0=禁用
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrParamInvalid, "请提供状态")
		return
	}
	if err := h.roleSvc.UpdateStatus(id, *req.Status); err != nil {
		response.Fail(c, response.ErrSystemError, err.Error())
		return
	}
	response.Success(c, gin.H{})
}

// AssignMenus 为角色分配菜单（替换式，会清除旧关联再写入新关联）。
// PUT /api/v1/roles/:id/menus
func (h *RoleHandler) AssignMenus(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		MenuIDs []uint64 `json:"menu_ids"` // 菜单 ID 列表
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrParamInvalid, "参数错误")
		return
	}
	if err := h.roleSvc.AssignMenus(id, req.MenuIDs); err != nil {
		response.Fail(c, response.ErrSystemError, err.Error())
		return
	}
	response.Success(c, gin.H{})
}

// GetMenus 获取角色拥有的菜单列表。
// GET /api/v1/roles/:id/menus
func (h *RoleHandler) GetMenus(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	menus, err := h.roleSvc.GetMenus(id)
	if err != nil {
		response.Fail(c, response.ErrSystemError, err.Error())
		return
	}
	response.Success(c, menus)
}

// AssignPermissions 为角色分配 Casbin 权限策略（先清除后添加的替换模式）。
// 每个 PermItem 对应一条 Casbin p 规则：p, role_code, path, method。
// PUT /api/v1/roles/:id/permissions
func (h *RoleHandler) AssignPermissions(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		Permissions []service.PermItem `json:"permissions"` // 权限项列表
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrParamInvalid, "参数错误")
		return
	}
	if err := h.roleSvc.AssignPermissions(id, req.Permissions); err != nil {
		response.Fail(c, response.ErrSystemError, err.Error())
		return
	}
	response.Success(c, gin.H{})
}
