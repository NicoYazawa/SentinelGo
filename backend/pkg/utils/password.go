// Package utils 提供通用工具函数，包括密码哈希、参数校验翻译、菜单树构建等。
package utils

import "golang.org/x/crypto/bcrypt"

// HashPassword 使用 bcrypt 算法对明文密码进行哈希。
// bcrypt.DefaultCost=10，计算耗时约 100ms，兼顾安全性与性能。
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPasswordHash 验证明文密码与 bcrypt 哈希是否匹配。
// 用于登录时密码校验和修改密码时的旧密码验证。
func CheckPasswordHash(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
