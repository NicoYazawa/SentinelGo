package handler

import (
	"strconv"

	"sentinelgo/internal/model"
	"sentinelgo/internal/service"
	"sentinelgo/pkg/response"

	"github.com/gin-gonic/gin"
)

// MenuHandler 是菜单管理的 HTTP 控制器，提供菜单的 CRUD 和用户菜单树查询。
type MenuHandler struct {
	menuSvc *service.MenuService
}

// NewMenuHandler 创建菜单 Handler（Wire Provider）。
func NewMenuHandler(menuSvc *service.MenuService) *MenuHandler {
	return &MenuHandler{menuSvc: menuSvc}
}

// List 返回完整的菜单树（扁平数据经 BuildMenuTree 转换为嵌套结构）。
// GET /api/v1/menus
func (h *MenuHandler) List(c *gin.Context) {
	tree, err := h.menuSvc.ListTree()
	if err != nil {
		response.Fail(c, response.ErrSystemError, err.Error())
		return
	}
	response.Success(c, tree)
}

// Get 按 ID 查询单个菜单。
// GET /api/v1/menus/:id
func (h *MenuHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	menu, err := h.menuSvc.GetByID(id)
	if err != nil {
		response.Fail(c, response.ErrSystemError, "菜单不存在")
		return
	}
	response.Success(c, menu)
}

// Create 创建新菜单。
// POST /api/v1/menus
func (h *MenuHandler) Create(c *gin.Context) {
	var menu model.Menu
	if err := c.ShouldBindJSON(&menu); err != nil {
		response.Fail(c, response.ErrParamInvalid, "参数错误")
		return
	}
	if err := h.menuSvc.Create(&menu); err != nil {
		response.Fail(c, response.ErrSystemError, err.Error())
		return
	}
	response.Success(c, menu)
}

// Update 更新菜单信息（部分更新，如 ParentID 为 0 则不更新该字段）。
// PUT /api/v1/menus/:id
func (h *MenuHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var menu model.Menu
	if err := c.ShouldBindJSON(&menu); err != nil {
		response.Fail(c, response.ErrParamInvalid, "参数错误")
		return
	}
	if err := h.menuSvc.Update(id, &menu); err != nil {
		response.Fail(c, response.ErrSystemError, err.Error())
		return
	}
	response.Success(c, gin.H{})
}

// Delete 删除菜单。
// DELETE /api/v1/menus/:id
func (h *MenuHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.menuSvc.Delete(id); err != nil {
		response.Fail(c, response.ErrSystemError, err.Error())
		return
	}
	response.Success(c, gin.H{})
}

// UpdateStatus 更新菜单启用/禁用状态。
// PUT /api/v1/menus/:id/status
func (h *MenuHandler) UpdateStatus(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		Status int8 `json:"status"` // 1=启用, 0=禁用
	}
	c.ShouldBindJSON(&req)
	if err := h.menuSvc.UpdateStatus(id, req.Status); err != nil {
		response.Fail(c, response.ErrSystemError, err.Error())
		return
	}
	response.Success(c, gin.H{})
}

// GetUserMenus 获取当前登录用户可访问的菜单树。
// 根据用户 ID 通过角色-菜单关联查询，结果为空时兜底返回全部菜单。
// GET /api/v1/menus/user
func (h *MenuHandler) GetUserMenus(c *gin.Context) {
	userID, _ := c.Get("current_user_id")
	id, _ := userID.(uint64)

	tree, err := h.menuSvc.GetUserMenus(id)
	if err != nil {
		response.Fail(c, response.ErrMenuNotAvailable, "获取用户菜单失败")
		return
	}
	response.Success(c, tree)
}
