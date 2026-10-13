package repository

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/plugin/dbresolver"

	"sentinelgo/internal/model"
)

// UserRepository 是用户数据访问层，继承 BaseRepository 的通用 CRUD 能力。
type UserRepository struct {
	*BaseRepository[model.User]          // 嵌入泛型基类，自动获得 Create/Update/Delete/FindByID/List
	db                          *gorm.DB // 保留 db 引用，用于扩展自定义查询
}

// NewUserRepository 创建用户 Repository（Wire Provider）。
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{
		BaseRepository: NewBaseRepository[model.User](db),
		db:             db,
	}
}

// FindByUsername 按用户名精确查询。
func (r *UserRepository) FindByUsername(username string) (*model.User, error) {
	var user model.User
	err := r.db.Clauses(dbresolver.Write).Where("username = ?", username).First(&user).Error
	return &user, err
}

func (r *UserRepository) CreateAccount(user *model.User, identity *model.UserAuth) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var role model.Role
		if err := tx.Where("code = ? AND status = ?", "user", 1).First(&role).Error; err != nil {
			return err
		}
		user.Roles = nil
		if err := tx.Create(user).Error; err != nil {
			return err
		}
		identity.UserID = user.ID
		if err := tx.Create(identity).Error; err != nil {
			return err
		}
		return NewUserRepository(tx).AssignRoles(user.ID, []uint64{role.ID})
	})
}

// UpdateStatus 更新用户启用/禁用状态。
func (r *UserRepository) UpdateStatus(id uint64, status int8) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.User{}).Where("id = ?", id).Update("status", status).Error; err != nil {
			return err
		}
		if status == 0 {
			return revokeUserSessions(tx, id)
		}
		return nil
	})
}

// UpdatePassword 更新用户密码哈希。
func (r *UserRepository) UpdatePassword(id uint64, password string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.User{}).Where("id = ?", id).Update("password", password).Error; err != nil {
			return err
		}
		var user model.User
		if err := tx.First(&user, id).Error; err != nil {
			return err
		}
		identity := model.UserAuth{UserID: id, IdentityType: "password", Identifier: user.Username, Credential: password}
		if err := tx.Where("user_id = ? AND identity_type = ?", id, "password").Assign(map[string]any{"credential": password, "identifier": user.Username}).FirstOrCreate(&identity).Error; err != nil {
			return err
		}
		return revokeUserSessions(tx, id)
	})
}

// CompleteBootstrapPassword atomically consumes the temporary credential and revokes all sessions.
func (r *UserRepository) CompleteBootstrapPassword(id uint64, sessionID, oldHash, newHash string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, id).Error; err != nil {
			return err
		}
		if user.Status != 1 || !user.PasswordChangeRequired || user.Password != oldHash {
			return errors.New("初始化状态已变化，请重新登录")
		}
		if err := NewAuthSessionRepository(tx).Validate(sessionID, id); err != nil {
			return err
		}
		if err := tx.Model(&user).Updates(map[string]any{"password": newHash, "password_change_required": false, "bootstrap_completed": true}).Error; err != nil {
			return err
		}
		identity := model.UserAuth{UserID: id, IdentityType: "password", Identifier: user.Username, Credential: newHash}
		if err := tx.Where("user_id = ? AND identity_type = ?", id, "password").Assign(map[string]any{"credential": newHash, "identifier": user.Username}).FirstOrCreate(&identity).Error; err != nil {
			return err
		}
		return revokeUserSessions(tx, id)
	})
}

// UpdateDetails never writes credentials or account status from stale snapshots.
func (r *UserRepository) UpdateDetails(id uint64, updates map[string]any) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.User{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return err
		}
		if username, ok := updates["username"]; ok {
			return tx.Model(&model.UserAuth{}).Where("user_id = ? AND identity_type = ?", id, "password").Update("identifier", username).Error
		}
		return nil
	})
}

func revokeUserSessions(db *gorm.DB, userID uint64) error {
	return db.Model(&model.AuthSession{}).Where("user_id = ?", userID).Update("revoked", true).Error
}

func (r *UserRepository) FindForAuth(id uint64) (*model.User, error) {
	var user model.User
	err := r.db.Clauses(dbresolver.Write).First(&user, id).Error
	return &user, err
}

func (r *UserRepository) GetEnabledRoles(id uint64) ([]model.Role, error) {
	var roles []model.Role
	err := r.db.Clauses(dbresolver.Write).Model(&model.Role{}).
		Joins("JOIN user_roles ur ON ur.role_id = sys_role.id").
		Where("ur.user_id = ? AND sys_role.status = ?", id, 1).Find(&roles).Error
	return roles, err
}

func (r *UserRepository) Delete(id uint64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := revokeUserSessions(tx, id); err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", id).Delete(&model.UserRole{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", id).Delete(&model.UserAuth{}).Error; err != nil {
			return err
		}
		if err := tx.Where("ptype = ? AND v0 = ?", "g", fmt.Sprint(id)).Delete(&model.CasbinRule{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.User{}, id).Error
	})
}

// UpdateAvatar 更新用户头像 URL。
func (r *UserRepository) UpdateAvatar(id uint64, avatarURL string) error {
	return r.db.Model(&model.User{}).Where("id = ?", id).Update("avatar", avatarURL).Error
}

// AssignRoles 替换用户的角色分配（先清空再设置）。
// 使用 GORM Association Replace，自动管理 user_roles 中间表。
func (r *UserRepository) AssignRoles(userID uint64, roleIDs []uint64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.First(&user, userID).Error; err != nil {
			return err
		}
		var roles []model.Role
		if len(roleIDs) > 0 {
			if err := tx.Where("id IN ?", roleIDs).Find(&roles).Error; err != nil {
				return err
			}
			seen := map[uint64]bool{}
			for _, id := range roleIDs {
				seen[id] = true
			}
			if len(roles) != len(seen) {
				return errors.New("角色不存在")
			}
		}
		if err := tx.Model(&user).Association("Roles").Replace(roles); err != nil {
			return err
		}
		if err := tx.Where("ptype = ? AND v0 = ?", "g", fmt.Sprint(userID)).Delete(&model.CasbinRule{}).Error; err != nil {
			return err
		}
		for _, role := range roles {
			if err := tx.Create(&model.CasbinRule{Ptype: "g", V0: fmt.Sprint(userID), V1: role.Code}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// GetUserRoles 获取用户拥有的所有角色（预加载 Roles 关联）。
func (r *UserRepository) GetUserRoles(userID uint64) ([]model.Role, error) {
	var user model.User
	if err := r.db.Preload("Roles").First(&user, userID).Error; err != nil {
		return nil, err
	}
	return user.Roles, nil
}

// ListWithKeyword 分页 + 关键词搜索用户列表。
// keyword 同时匹配用户名、昵称、邮箱字段。
func (r *UserRepository) ListWithKeyword(keyword string, page, pageSize int) ([]model.User, int64, error) {
	var users []model.User
	var total int64

	q := r.db.Model(&model.User{})
	if keyword != "" {
		// LIKE 模糊匹配多字段
		q = q.Where("username LIKE ? OR nickname LIKE ? OR email LIKE ?", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}

	// 先查总数
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	err := q.Offset(offset).Limit(pageSize).Order("id DESC").Find(&users).Error
	return users, total, err
}
