package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"

	"sentinelgo/internal/model"
)

var ErrSessionInvalid = errors.New("登录会话已失效，请重新登录")

type AuthSessionRepository struct{ db *gorm.DB }

func NewAuthSessionRepository(db *gorm.DB) *AuthSessionRepository {
	return &AuthSessionRepository{db: db}
}

func (r *AuthSessionRepository) Create(session *model.AuthSession) error {
	return r.db.Create(session).Error
}

func (r *AuthSessionRepository) Validate(id string, userID uint64) error {
	if id == "" {
		return ErrSessionInvalid
	}
	var count int64
	err := r.db.Clauses(dbresolver.Write).Model(&model.AuthSession{}).
		Where("id = ? AND user_id = ? AND revoked = ? AND expires_at > ?", id, userID, false, time.Now()).Count(&count).Error
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrSessionInvalid
	}
	return nil
}

// Rotate uses a compare-and-swap so a refresh token can succeed only once.
func (r *AuthSessionRepository) Rotate(id string, userID uint64, oldJTI, newJTI string) error {
	result := r.db.Model(&model.AuthSession{}).
		Where("id = ? AND user_id = ? AND refresh_jti = ? AND revoked = ? AND expires_at > ?", id, userID, oldJTI, false, time.Now()).
		Update("refresh_jti", newJTI)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrSessionInvalid
	}
	return nil
}

func (r *AuthSessionRepository) Revoke(id string, userID uint64) error {
	return r.db.Model(&model.AuthSession{}).Where("id = ? AND user_id = ?", id, userID).Update("revoked", true).Error
}
