// Package casbin 封装 Casbin RBAC 权限引擎的初始化与常用操作。
//
// 基于 Casbin SyncedEnforcer（线程安全版本），使用 GORM 适配器
// 将策略规则持久化到 MySQL，支持运行时动态加载策略。
//
// 权限模型：sub (用户角色) → obj (请求路径) → act (HTTP 方法)
// 策略示例：p, admin, /api/v1/users, GET  表示 admin 角色可以 GET /api/v1/users
package casbin

import (
	"fmt"

	"github.com/casbin/casbin/v3"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"
)

// Enforcer 是全局 Casbin 同步执行器实例。
// SyncedEnforcer 内部使用读写锁保护，支持高并发场景。
var Enforcer *casbin.SyncedEnforcer

// Init 初始化 Casbin 权限引擎。
//
// 参数：
//   - db: GORM 数据库实例，用于持久化策略规则
//   - modelPath: Casbin RBAC 模型配置文件路径（如 config/rbac_model.conf）
func Init(db *gorm.DB, modelPath string) {
	// 使用 GORM 适配器，策略规则存储在 casbin_rule 表中
	adapter, err := gormadapter.NewAdapterByDB(db.Clauses(dbresolver.Write))
	if err != nil {
		panic(fmt.Sprintf("Casbin 适配器初始化失败: %v", err))
	}

	enforcer, err := casbin.NewSyncedEnforcer(modelPath, adapter)
	if err != nil {
		panic(fmt.Sprintf("Casbin Enforcer 初始化失败: %v", err))
	}

	// 开启自动保存，每次 AddPolicy/RemovePolicy 立即持久化到数据库
	enforcer.EnableAutoSave(true)
	Enforcer = enforcer
}

// CheckPermission 检查指定主体对资源的操作权限。
// sub: 用户标识（如角色名 "admin"）
// obj: 资源路径（如 "/api/v1/users"）
// act: 操作类型（如 "GET", "POST", "DELETE"）
func CheckPermission(sub, obj, act string) (bool, error) {
	return Enforcer.Enforce(sub, obj, act)
}

// AddRoleForUser 为用户分配角色，建立 g 规则（user → role 映射）。
func AddRoleForUser(user, role string) (bool, error) {
	return Enforcer.AddRoleForUser(user, role)
}

// RemoveRoleForUser 移除用户的角色分配。
func RemoveRoleForUser(user, role string) (bool, error) {
	return Enforcer.DeleteRoleForUser(user, role)
}

// GetRolesForUser 获取用户拥有的所有角色。
func GetRolesForUser(user string) ([]string, error) {
	return Enforcer.GetRolesForUser(user)
}

// AddPolicy 添加权限策略规则（p 规则：role → path → method）。
func AddPolicy(role, path, method string) (bool, error) {
	return Enforcer.AddPolicy(role, path, method)
}

// RemovePolicy 移除指定权限策略规则。
func RemovePolicy(role, path, method string) (bool, error) {
	return Enforcer.RemovePolicy(role, path, method)
}

// DeletePermissionsForUser 删除指定用户的所有权限。
func DeletePermissionsForUser(user string) (bool, error) {
	return Enforcer.DeletePermissionsForUser(user)
}

// LoadPolicy 从数据库重新加载所有策略规则。
// 适用于手动修改了 casbin_rule 表后需刷新内存中的策略缓存。
func LoadPolicy() error {
	return Enforcer.LoadPolicy()
}
