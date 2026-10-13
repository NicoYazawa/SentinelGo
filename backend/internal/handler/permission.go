package handler

import (
	"sentinelgo/internal/model"
	"sentinelgo/internal/service"
	"sentinelgo/pkg/response"

	"github.com/gin-gonic/gin"
)

// PermissionHandler 是权限定义管理的 HTTP 控制器，提供权限定义的 CRUD。
// 注意：此模块管理权限定义（Permission 表），与 Casbin 策略规则（p 规则）是两个维度。
// 权限定义用于前端展示和权限选择，Casbin 策略用于运行时鉴权。
type PermissionHandler struct {
	permSvc *service.PermissionService
}

// NewPermissionHandler 创建权限 Handler（Wire Provider）。
func NewPermissionHandler(permSvc *service.PermissionService) *PermissionHandler {
	return &PermissionHandler{permSvc: permSvc}
}

// List 多条件分页查询权限定义列表，支持关键词、方法、状态过滤。
// GET /api/v1/permissions
func (h *PermissionHandler) List(c *gin.Context) {
	var req service.PermissionListReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, response.ErrParamInvalid, "参数错误")
		return
	}

	perms, total, err := h.permSvc.List(req)
	if err != nil {
		response.Fail(c, response.ErrSystemError, err.Error())
		return
	}
	response.Success(c, gin.H{"list": perms, "total": total})
}

// Get 按 ID 查询单个权限定义。
// GET /api/v1/permissions/:id
func (h *PermissionHandler) Get(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	perm, err := h.permSvc.GetByID(id)
	if err != nil {
		response.Fail(c, response.ErrSystemError, "权限不存在")
		return
	}
	response.Success(c, perm)
}

// Create 创建新权限定义（默认状态启用）。
// POST /api/v1/permissions
func (h *PermissionHandler) Create(c *gin.Context) {
	var perm model.Permission
	if err := c.ShouldBindJSON(&perm); err != nil {
		response.Fail(c, response.ErrParamInvalid, "参数错误")
		return
	}
	if err := h.permSvc.Create(&perm); err != nil {
		response.Fail(c, response.ErrSystemError, err.Error())
		return
	}
	response.Success(c, perm)
}

// Update 更新权限定义（部分更新：path, method, description, status）。
// PUT /api/v1/permissions/:id
func (h *PermissionHandler) Update(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var perm model.Permission
	if err := c.ShouldBindJSON(&perm); err != nil {
		response.Fail(c, response.ErrParamInvalid, "参数错误")
		return
	}
	if err := h.permSvc.Update(id, &perm); err != nil {
		response.Fail(c, response.ErrSystemError, err.Error())
		return
	}
	response.Success(c, gin.H{})
}

// Delete 删除权限定义。
// DELETE /api/v1/permissions/:id
func (h *PermissionHandler) Delete(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	if err := h.permSvc.Delete(id); err != nil {
		response.Fail(c, response.ErrSystemError, err.Error())
		return
	}
	response.Success(c, gin.H{})
}
