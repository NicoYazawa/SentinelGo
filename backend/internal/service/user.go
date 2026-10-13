package service

import (
	"errors"

	"sentinelgo/internal/model"
	"sentinelgo/internal/repository"
	casbinpkg "sentinelgo/pkg/casbin"
	"sentinelgo/pkg/utils"
)

// UserService 是用户管理业务逻辑服务。
type UserService struct {
	userRepo *repository.UserRepository
	roleRepo *repository.RoleRepository
}

// NewUserService 创建用户服务（Wire Provider）。
func NewUserService(userRepo *repository.UserRepository, roleRepo *repository.RoleRepository) *UserService {
	return &UserService{userRepo: userRepo, roleRepo: roleRepo}
}

// UserListReq 用户列表查询请求参数。
type UserListReq struct {
	Keyword  string `form:"keyword"`   // 模糊搜索关键词（匹配用户名、昵称、邮箱）
	Page     int    `form:"page"`      // 页码（从 1 开始）
	PageSize int    `form:"page_size"` // 每页条数
}

// AssignRolesReq 用户角色分配请求参数。
type AssignRolesReq struct {
	RoleIDs []uint64 `json:"role_ids"` // 空列表表示撤销所有角色
}

// List 分页 + 关键词查询用户列表。
func (s *UserService) List(req UserListReq) ([]model.User, int64, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	return s.userRepo.ListWithKeyword(req.Keyword, req.Page, req.PageSize)
}

// GetByID 按 ID 查询用户。
func (s *UserService) GetByID(id uint64) (*model.User, error) {
	return s.userRepo.FindByID(id)
}

// Create 创建新用户，使用请求中的密码并初始化普通用户权限。
// 创建后自动赋予「普通用户」(user) 角色，并同步 Casbin 的 g 规则，
// 否则该用户因无任何角色映射，将无法访问个人中心等基础接口（报权限不足）。
func (s *UserService) Create(req *model.User) error {
	if req.Username == "" || req.Password == "" {
		return errors.New("用户名和密码不能为空")
	}
	hashedPwd, err := utils.HashPassword(req.Password)
	if err != nil {
		return err
	}
	req.Password = hashedPwd
	req.Status = 1
	req.ID = 0
	req.IsAdmin = false
	if err := s.userRepo.CreateAccount(req, &model.UserAuth{IdentityType: "password", Identifier: req.Username, Credential: hashedPwd}); err != nil {
		return err
	}
	return casbinpkg.LoadPolicy()
}

// Update 更新用户信息（仅更新有值的字段）。
func (s *UserService) Update(id uint64, req *model.User) error {
	if _, err := s.userRepo.FindByID(id); err != nil {
		return err
	}
	if req.Password != "" {
		if err := s.UpdatePassword(id, req.Password); err != nil {
			return err
		}
	}
	updates := map[string]any{}
	if req.Username != "" {
		updates["username"] = req.Username
	}
	if req.Nickname != "" {
		updates["nickname"] = req.Nickname
	}
	if req.Email != "" {
		updates["email"] = req.Email
	}
	if req.Phone != "" {
		updates["phone"] = req.Phone
	}
	return s.userRepo.UpdateDetails(id, updates)
}

// Delete 删除用户。
func (s *UserService) Delete(id uint64) error {
	if err := s.userRepo.Delete(id); err != nil {
		return err
	}
	return casbinpkg.LoadPolicy()
}

// UpdateStatus 更新用户启用/禁用状态。
func (s *UserService) UpdateStatus(id uint64, status int8) error {
	if status != 0 && status != 1 {
		return errors.New("状态必须为 0 或 1")
	}
	return s.userRepo.UpdateStatus(id, status)
}

// UpdatePassword 更新用户密码（需传入新密码明文，内部 bcrypt 哈希）。
func (s *UserService) UpdatePassword(id uint64, password string) error {
	if password == "" {
		return errors.New("密码不能为空")
	}
	hashedPwd, err := utils.HashPassword(password)
	if err != nil {
		return err
	}
	return s.userRepo.UpdatePassword(id, hashedPwd)
}

// AssignRoles 为用户分配角色（先清除旧角色，再设置新角色，同步更新 Casbin 策略）。
func (s *UserService) AssignRoles(userID uint64, roleIDs []uint64) error {
	if err := s.userRepo.AssignRoles(userID, roleIDs); err != nil {
		return err
	}
	return casbinpkg.LoadPolicy()
}

// GetRoles 获取用户拥有的所有角色。
func (s *UserService) GetRoles(userID uint64) ([]model.Role, error) {
	return s.userRepo.GetUserRoles(userID)
}

// UpdateProfile 用户自行修改个人信息（昵称、邮箱、手机、时区、界面语言）。
func (s *UserService) UpdateProfile(userID uint64, nickname, email, phone, timezone, locale string) error {
	if _, err := s.userRepo.FindForAuth(userID); err != nil {
		return err
	}
	updates := map[string]any{}
	if nickname != "" {
		updates["nickname"] = nickname
	}
	if email != "" {
		updates["email"] = email
	}
	if phone != "" {
		updates["phone"] = phone
	}
	if timezone != "" {
		updates["timezone"] = timezone
	}
	if locale != "" {
		updates["locale"] = locale
	}
	return s.userRepo.UpdateDetails(userID, updates)
}

// UpdateAvatar 更新用户头像 URL。
func (s *UserService) UpdateAvatar(userID uint64, avatarURL string) error {
	return s.userRepo.UpdateAvatar(userID, avatarURL)
}
