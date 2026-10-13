package repository

import (
	"sentinelgo/internal/model"

	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"
)

// UserAuthRepository 是多渠道认证凭据的数据访问层。
// 支持同一用户绑定多种登录方式（密码/手机/GitHub OAuth 等）。
type UserAuthRepository struct {
	db *gorm.DB
}

// NewUserAuthRepository 创建认证凭据 Repository。
func NewUserAuthRepository(db *gorm.DB) *UserAuthRepository {
	return &UserAuthRepository{db: db}
}

// FindByIdentity 按认证类型 + 标识符查询认证记录。
// 用于登录时校验密码/短信验证码/OAuth 绑定查找。
func (r *UserAuthRepository) FindByIdentity(identityType, identifier string) (*model.UserAuth, error) {
	var auth model.UserAuth
	err := r.db.Clauses(dbresolver.Write).Where("identity_type = ? AND identifier = ?", identityType, identifier).First(&auth).Error
	return &auth, err
}

// Create 新增认证记录（注册时创建密码认证，或绑定新渠道时创建）。
func (r *UserAuthRepository) Create(auth *model.UserAuth) error {
	return r.db.Create(auth).Error
}

// Delete 按主键删除认证记录。
func (r *UserAuthRepository) Delete(id uint64) error {
	return r.db.Delete(&model.UserAuth{}, id).Error
}

// ListByUserID 查询用户的所有绑定渠道。
func (r *UserAuthRepository) ListByUserID(userID uint64) ([]model.UserAuth, error) {
	var auths []model.UserAuth
	err := r.db.Where("user_id = ?", userID).Find(&auths).Error
	return auths, err
}

// DeleteByType 删除用户指定类型的认证绑定（如解绑 GitHub 登录）。
func (r *UserAuthRepository) DeleteByType(userID uint64, identityType string) error {
	return r.db.Where("user_id = ? AND identity_type = ?", userID, identityType).Delete(&model.UserAuth{}).Error
}
