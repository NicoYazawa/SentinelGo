package handler

import (
	"fmt"
	"path/filepath"
	"time"

	"sentinelgo/internal/model"
	"sentinelgo/internal/service"
	"sentinelgo/pkg/response"

	"github.com/gin-gonic/gin"
)

// avatarBaseDir 是头像上传的根目录，默认 "uploads"。
// 测试时可临时重定向到临时目录，避免污染仓库。
var avatarBaseDir = "uploads"

// UserHandler 是用户管理的 HTTP 控制器，提供用户的 CRUD、角色分配、个人中心、头像上传。
type UserHandler struct {
	userSvc *service.UserService
}

// NewUserHandler 创建用户 Handler（Wire Provider）。
func NewUserHandler(userSvc *service.UserService) *UserHandler {
	return &UserHandler{userSvc: userSvc}
}

// List 分页查询用户列表，支持关键词搜索和状态过滤。
// GET /api/v1/users
func (h *UserHandler) List(c *gin.Context) {
	var req service.UserListReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Fail(c, response.ErrParamInvalid, "参数错误")
		return
	}

	users, total, err := h.userSvc.List(req)
	if err != nil {
		response.Fail(c, response.ErrSystemError, err.Error())
		return
	}

	response.Success(c, gin.H{"list": users, "total": total})
}

// Get 按 ID 查询单个用户详情。
// GET /api/v1/users/:id
func (h *UserHandler) Get(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	user, err := h.userSvc.GetByID(id)
	if err != nil {
		response.Fail(c, response.ErrUserNotFound, "用户不存在")
		return
	}
	response.Success(c, user)
}

// Create 创建新用户。
// POST /api/v1/users
func (h *UserHandler) Create(c *gin.Context) {
	var req struct {
		model.User
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrParamInvalid, "参数错误")
		return
	}
	user := req.User
	user.Password = req.Password
	if err := h.userSvc.Create(&user); err != nil {
		response.Fail(c, response.ErrSystemError, err.Error())
		return
	}
	response.Success(c, user)
}

// Update 更新用户信息（部分更新，只更新传入的非零字段）。
// PUT /api/v1/users/:id
func (h *UserHandler) Update(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		model.User
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrParamInvalid, "参数错误")
		return
	}
	user := req.User
	user.Password = req.Password
	if err := h.userSvc.Update(id, &user); err != nil {
		response.Fail(c, response.ErrSystemError, err.Error())
		return
	}
	response.Success(c, gin.H{})
}

// Delete 删除指定用户。
// DELETE /api/v1/users/:id
func (h *UserHandler) Delete(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	if err := h.userSvc.Delete(id); err != nil {
		response.Fail(c, response.ErrSystemError, err.Error())
		return
	}
	response.Success(c, gin.H{})
}

// UpdateStatus 更新用户启用/禁用状态。
// PUT /api/v1/users/:id/status
func (h *UserHandler) UpdateStatus(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		Status *int8 `json:"status" binding:"required"` // 1=启用, 0=禁用
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrParamInvalid, "请提供状态")
		return
	}
	if err := h.userSvc.UpdateStatus(id, *req.Status); err != nil {
		response.Fail(c, response.ErrSystemError, err.Error())
		return
	}
	response.Success(c, gin.H{})
}

// UpdatePassword 修改指定用户的密码（管理员操作，会 bcrypt 加密）。
// PUT /api/v1/users/:id/password
func (h *UserHandler) UpdatePassword(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		Password string `json:"password" binding:"required"` // 新密码
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrParamInvalid, "请输入新密码")
		return
	}
	if err := h.userSvc.UpdatePassword(id, req.Password); err != nil {
		response.Fail(c, response.ErrSystemError, err.Error())
		return
	}
	response.Success(c, gin.H{})
}

// AssignRoles 为用户分配角色（替换式，会同步更新 Casbin 策略）。
// PUT /api/v1/users/:id/roles
func (h *UserHandler) AssignRoles(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var req service.AssignRolesReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrParamInvalid, "请提供角色列表")
		return
	}
	if err := h.userSvc.AssignRoles(id, req.RoleIDs); err != nil {
		response.Fail(c, response.ErrSystemError, err.Error())
		return
	}
	response.Success(c, gin.H{})
}

// GetRoles 获取指定用户的角色列表。
// GET /api/v1/users/:id/roles
func (h *UserHandler) GetRoles(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	roles, err := h.userSvc.GetRoles(id)
	if err != nil {
		response.Fail(c, response.ErrSystemError, err.Error())
		return
	}
	response.Success(c, roles)
}

// GetCurrentUser 获取当前登录用户的详细信息（含角色列表）。
// 用户 ID 从 JWT 中间件注入的 current_user_id 中提取。
// GET /api/v1/users/me
func (h *UserHandler) GetCurrentUser(c *gin.Context) {
	id, ok := currentUserID(c)
	if !ok {
		return
	}
	user, err := h.userSvc.GetByID(id)
	if err != nil {
		response.Fail(c, response.ErrUserNotFound, "用户不存在")
		return
	}
	roles, err := h.userSvc.GetRoles(id)
	if err != nil {
		response.Fail(c, response.ErrSystemError, err.Error())
		return
	}
	response.Success(c, gin.H{
		"user":  user,
		"roles": roles,
	})
}

// ---- 个人中心 ----

// UpdateProfileReq 个人信息更新请求，所有字段可选，仅更新传入的字段。
type UpdateProfileReq struct {
	Nickname string `json:"nickname"` // 昵称
	Email    string `json:"email"`    // 邮箱
	Phone    string `json:"phone"`    // 手机号
	Timezone string `json:"timezone"` // 个人时区（IANA，如 Asia/Shanghai）
	Locale   string `json:"locale"`   // 个人界面语言（如 zh-CN/en-US）
}

// GetProfile 获取当前用户的个人信息。
// GET /api/v1/profile
func (h *UserHandler) GetProfile(c *gin.Context) {
	id, ok := currentUserID(c)
	if !ok {
		return
	}
	user, err := h.userSvc.GetByID(id)
	if err != nil {
		response.Fail(c, response.ErrUserNotFound, "用户不存在")
		return
	}
	response.Success(c, user)
}

// UpdateProfile 更新当前用户的个人信息（昵称、邮箱、手机号）。
// PUT /api/v1/profile
func (h *UserHandler) UpdateProfile(c *gin.Context) {
	id, ok := currentUserID(c)
	if !ok {
		return
	}
	var req UpdateProfileReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrParamInvalid, "参数错误")
		return
	}
	if err := h.userSvc.UpdateProfile(id, req.Nickname, req.Email, req.Phone, req.Timezone, req.Locale); err != nil {
		response.Fail(c, response.ErrSystemError, err.Error())
		return
	}
	response.Success(c, gin.H{})
}

// UploadAvatar 上传当前用户的头像文件。
// 文件保存到 uploads/avatars/ 目录，文件名格式：{userID}_{timestamp}{ext}。
// POST /api/v1/profile/avatar
func (h *UserHandler) UploadAvatar(c *gin.Context) {
	id, ok := currentUserID(c)
	if !ok {
		return
	}

	// 从 multipart form 中提取 avatar 文件字段
	file, err := c.FormFile("avatar")
	if err != nil {
		response.Fail(c, response.ErrParamInvalid, "请选择头像文件")
		return
	}

	// 构造唯一文件名：用户ID + 时间戳 + 原始扩展名
	filename := fmt.Sprintf("%d_%d%s", id, time.Now().Unix(), filepath.Ext(file.Filename))
	savePath := filepath.Join(avatarBaseDir, "avatars", filename)
	if err := c.SaveUploadedFile(file, savePath); err != nil {
		response.Fail(c, response.ErrSystemError, "头像上传失败")
		return
	}

	// 更新用户头像 URL（相对路径，前端拼接域名访问）
	avatarURL := "/uploads/avatars/" + filename
	if err := h.userSvc.UpdateAvatar(id, avatarURL); err != nil {
		response.Fail(c, response.ErrSystemError, "更新头像失败")
		return
	}
	response.Success(c, gin.H{"avatar_url": avatarURL})
}
