// Package token 提供 JWT 令牌的生成、解析、校验与黑名单管理。
//
// 双令牌机制：
//   - Access Token：短期有效（15 分钟），用于 API 鉴权
//   - Refresh Token：长期有效（7 天），用于无感刷新 Access Token
//
// 黑名单：登出或刷新时，将旧 Access Token 的 JTI 写入 Redis 黑名单，
// 配合 JWTAuth 中间件拦截已注销但尚未过期的 Token。
package token

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Token 过期时间常量。
const (
	AccessTokenTTL  = 15 * time.Minute   // Access Token 有效期
	RefreshTokenTTL = 7 * 24 * time.Hour // Refresh Token 有效期
)

// JWTSecret 是 JWT 签名密钥，由 main.go 在启动时通过 token.Init 注入。
var JWTSecret []byte

// RDB 是 Redis 客户端，用于 Token 黑名单存取。
var RDB *redis.Client

// Init 初始化 token 包的全局依赖。
func Init(secret string, rdb *redis.Client) {
	JWTSecret = []byte(secret)
	RDB = rdb
}

// CustomClaims 是自定义 JWT Claims，扩展了标准 RegisteredClaims。
type CustomClaims struct {
	UserID    uint64   `json:"user_id"`  // 用户 ID
	Username  string   `json:"username"` // 用户名
	Roles     []string `json:"roles"`    // 用户角色列表
	Locale    string   `json:"locale"`   // 用户偏好语言（如 zh-CN/en-US），供后端按语言返回提示
	JTIShort  string   `json:"jti"`      // 完整 UUID（字段名保留兼容性）
	TokenType string   `json:"type"`     // 令牌类型："access" 或 "refresh"
	SessionID string   `json:"sid"`      // 数据库中的登录会话
	jwt.RegisteredClaims
}

// GenerateAccessToken 生成短期 Access Token（15 分钟有效）。
func GenerateAccessToken(userID uint64, username string, roles []string, locale string, sessionIDs ...string) (string, error) {
	jti := uuid.New().String()
	claims := CustomClaims{
		UserID:    userID,
		Username:  username,
		Roles:     roles,
		Locale:    locale,
		JTIShort:  jti,
		TokenType: "access",
		SessionID: sessionID(sessionIDs),
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(AccessTokenTTL)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(JWTSecret)
}

// GenerateRefreshToken 生成长期 Refresh Token（7 天有效）。
func GenerateRefreshToken(userID uint64, username string, roles []string, locale string, sessionIDs ...string) (string, error) {
	jti := uuid.New().String()
	claims := CustomClaims{
		UserID:    userID,
		Username:  username,
		Roles:     roles,
		Locale:    locale,
		JTIShort:  jti,
		TokenType: "refresh",
		SessionID: sessionID(sessionIDs),
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(RefreshTokenTTL)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(JWTSecret)
}

// ParseToken 解析并验证 JWT Token 字符串，返回 Claims。
// 验证失败时返回 error（过期、签名不匹配等）。
func ParseToken(tokenString string) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (any, error) {
		return JWTSecret, nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, jwt.ErrSignatureInvalid
}

// BlacklistToken 将指定 JTI 加入 Redis 黑名单。
// ttl 应与 Access Token 剩余有效期一致，到期后自动清理。
func BlacklistToken(jti string, ttl time.Duration) error {
	if RDB == nil {
		return nil
	}
	return RDB.Set(context.Background(), "token:blacklist:"+jti, "1", ttl).Err()
}

// IsBlacklisted 检查 JTI 是否在黑名单中。
// 用于 JWTAuth 中间件拦截已注销的 Token。
func IsBlacklisted(jti string) bool {
	if RDB == nil {
		return false
	}
	exists, _ := RDB.Exists(context.Background(), "token:blacklist:"+jti).Result()
	return exists > 0
}

// TokenPair 是一对 Access + Refresh Token，用于登录接口响应。
type TokenPair struct {
	AccessToken  string `json:"access_token"`  // 访问令牌
	RefreshToken string `json:"refresh_token"` // 刷新令牌
	ExpiresIn    int64  `json:"expires_in"`    // Access Token 剩余有效秒数
}

// GenerateTokenPair 同时生成 Access Token 和 Refresh Token。
func GenerateTokenPair(userID uint64, username string, roles []string, locale string, sessionIDs ...string) (*TokenPair, error) {
	access, err := GenerateAccessToken(userID, username, roles, locale, sessionIDs...)
	if err != nil {
		return nil, fmt.Errorf("生成 AccessToken 失败: %w", err)
	}
	refresh, err := GenerateRefreshToken(userID, username, roles, locale, sessionIDs...)
	if err != nil {
		return nil, fmt.Errorf("生成 RefreshToken 失败: %w", err)
	}
	return &TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int64(AccessTokenTTL.Seconds()),
	}, nil
}

func sessionID(ids []string) string {
	if len(ids) > 0 {
		return ids[0]
	}
	return ""
}
