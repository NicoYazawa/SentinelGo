// Package model 定义 GORM 数据库实体（Entity），与 MySQL 表结构一一对应。
//
// 命名约定：
//   - 结构体名使用单数大驼峰，TableName() 返回 snake_case 复数表名
//   - json:"-" 标签隐藏敏感字段（密码、凭证等），序列化时自动排除
//   - gorm:"many2many:xxx" 定义多对多关联表名
//   - BaseModel 嵌入提供 ID / CreatedAt / UpdatedAt 三个通用字段
package model

import "time"

// BaseModel 是所有实体的基础结构，提供通用主键和时间戳字段。
// 通过 GORM 的 CreatedAt/UpdatedAt 钩子自动维护时间。
type BaseModel struct {
	ID        uint64    `gorm:"primarykey" json:"id"` // 自增主键
	CreatedAt time.Time `json:"created_at"`           // 创建时间（GORM 自动填充）
	UpdatedAt time.Time `json:"updated_at"`           // 更新时间（GORM 自动更新）
}

// User 系统用户实体，对应 sys_user 表。
// 与 Role 多对多关联（user_roles 中间表）。
type User struct {
	BaseModel
	Username               string `gorm:"size:50;not null;uniqueIndex" json:"username"`  // 登录用户名（唯一）
	Password               string `gorm:"size:255;not null" json:"-"`                    // bcrypt 密码哈希（不序列化到 JSON）
	Nickname               string `gorm:"size:50" json:"nickname"`                       // 显示昵称
	Email                  string `gorm:"size:100" json:"email"`                         // 邮箱
	Phone                  string `gorm:"size:20" json:"phone"`                          // 手机号
	Avatar                 string `gorm:"size:255" json:"avatar"`                        // 头像 URL
	Status                 int8   `gorm:"default:1;comment:1:启用 0:禁用" json:"status"`     // 状态：1=启用 0=禁用
	IsAdmin                bool   `gorm:"default:false" json:"is_admin"`                 // 是否为超级管理员
	Timezone               string `gorm:"size:50;default:Asia/Shanghai" json:"timezone"` // 个人时区（IANA，如 Asia/Shanghai），前端按用户时区展示时间
	Locale                 string `gorm:"size:10;default:zh-CN" json:"locale"`           // 个人界面语言（如 zh-CN/en-US），登录后界面与后端提示随其切换
	Roles                  []Role `gorm:"many2many:user_roles" json:"roles,omitempty"`   // 关联角色列表
	PasswordChangeRequired bool   `gorm:"not null;default:false" json:"password_change_required"`
	BootstrapCompleted     bool   `gorm:"not null;default:false" json:"-"`
}

func (User) TableName() string { return "sys_user" }

// Role 角色实体，对应 sys_role 表。
// 与 Menu 多对多关联（role_menus 中间表）。
type Role struct {
	BaseModel
	Name   string `gorm:"size:50;not null" json:"name"`                // 角色显示名称
	Code   string `gorm:"size:50;not null;uniqueIndex" json:"code"`    // 角色编码（唯一，如 admin, editor）
	Desc   string `gorm:"size:100" json:"desc"`                        // 角色描述
	Status int8   `gorm:"default:1" json:"status"`                     // 状态：1=启用 0=禁用
	Menus  []Menu `gorm:"many2many:role_menus" json:"menus,omitempty"` // 关联菜单列表
}

func (Role) TableName() string { return "sys_role" }

// Menu 系统菜单/权限资源实体，对应 sys_menu 表。
// Type 字段区分：1=目录 2=菜单 3=按钮。
// Children 为 gorm:"-" 忽略字段，由 BuildMenuTree 工具函数填充。
type Menu struct {
	BaseModel
	ParentID  uint64  `gorm:"index;default:0" json:"parent_id"`             // 父菜单 ID（0 为顶级）
	Name      string  `gorm:"size:50;not null" json:"name"`                 // 菜单名称
	Path      string  `gorm:"size:255" json:"path"`                         // 前端路由路径
	Component string  `gorm:"size:255" json:"component"`                    // 前端组件路径
	Icon      string  `gorm:"size:50" json:"icon"`                          // 菜单图标
	Sort      int     `gorm:"default:0" json:"sort"`                        // 排序号（越小越靠前）
	Type      int     `gorm:"default:1;comment:1:目录 2:菜单 3:按钮" json:"type"` // 菜单类型
	Perms     string  `gorm:"size:100" json:"perms"`                        // 权限标识（如 user:list）
	Status    int8    `gorm:"default:1" json:"status"`                      // 状态：1=启用 0=禁用
	Hidden    bool    `gorm:"default:false" json:"hidden"`                  // 是否隐藏（不影响权限，仅前端不显示）
	Children  []*Menu `gorm:"-" json:"children,omitempty"`                  // 子菜单（由 BuildMenuTree 填充，不存库）
}

func (Menu) TableName() string { return "sys_menu" }

// Permission 接口权限定义实体，对应 sys_permission 表。
type Permission struct {
	BaseModel
	Path        string `gorm:"size:255;not null" json:"path"`  // API 路径
	Method      string `gorm:"size:10;not null" json:"method"` // HTTP 方法
	Description string `gorm:"size:255" json:"description"`    // 权限描述
	Status      int8   `gorm:"default:1" json:"status"`        // 状态：1=启用 0=禁用
}

func (Permission) TableName() string { return "sys_permission" }

// CasbinRule 是 Casbin GORM 适配器的策略存储表。
// 字段 V0-V5 对应 Casbin 策略规则的不同部分（subject, object, action 等）。
type CasbinRule struct {
	ID    uint64 `gorm:"primarykey;autoIncrement" json:"id"`
	Ptype string `gorm:"size:100;not null" json:"ptype"` // 策略类型：p（权限规则）/ g（角色分配）
	V0    string `gorm:"size:100" json:"v0"`             // subject（用户/角色）
	V1    string `gorm:"size:100" json:"v1"`             // object（资源路径）
	V2    string `gorm:"size:100" json:"v2"`             // action（操作：GET/POST/PUT/DELETE）
	V3    string `gorm:"size:100" json:"v3"`
	V4    string `gorm:"size:100" json:"v4"`
	V5    string `gorm:"size:100" json:"v5"`
}

func (CasbinRule) TableName() string { return "casbin_rule" }

// OperationLog 操作日志实体，对应 sys_operation_log 表。
type OperationLog struct {
	BaseModel
	UserID    uint64 `gorm:"index" json:"user_id"`          // 操作用户 ID
	Username  string `gorm:"size:50" json:"username"`       // 操作用户名
	Module    string `gorm:"size:32;index" json:"module"`   // 业务模块（用户/角色/菜单...）
	Operation string `gorm:"size:255" json:"operation"`     // 操作描述
	Method    string `gorm:"size:10" json:"method"`         // 请求方法
	Path      string `gorm:"size:255" json:"path"`          // 请求路径
	IP        string `gorm:"size:50" json:"ip"`             // 客户端 IP
	Status    int    `json:"status"`                        // 操作结果状态：1=成功 0=失败（勿加 default 标签，否则零值 0 会被 GORM 跳过而误存为 1）
	ErrorMsg  string `gorm:"size:500" json:"error_msg"`     // 错误信息
	UserAgent string `gorm:"size:255" json:"user_agent"`    // 客户端 User-Agent
	TraceID   string `gorm:"size:64;index" json:"trace_id"` // 请求链路追踪 ID
}

func (OperationLog) TableName() string { return "sys_operation_log" }

// UserRole 用户-角色多对多关联中间表，对应 user_roles 表。
type UserRole struct {
	UserID uint64 `gorm:"primaryKey" json:"user_id"`
	RoleID uint64 `gorm:"primaryKey" json:"role_id"`
}

func (UserRole) TableName() string { return "user_roles" }

// RoleMenu 角色-菜单多对多关联中间表，对应 role_menus 表。
type RoleMenu struct {
	RoleID uint64 `gorm:"primaryKey" json:"role_id"`
	MenuID uint64 `gorm:"primaryKey" json:"menu_id"`
}

func (RoleMenu) TableName() string { return "role_menus" }

// UserAuth 多渠道认证凭据实体，对应 user_auths 表。
// 支持同一用户通过多种方式登录（密码/手机/GitHub/微信等）。
type UserAuth struct {
	BaseModel
	UserID       uint64 `gorm:"index;not null" json:"user_id"`                                                                            // 关联的用户 ID
	IdentityType string `gorm:"size:20;not null;uniqueIndex:idx_auth_identity;comment:password/phone/github/wechat" json:"identity_type"` // 认证类型
	Identifier   string `gorm:"size:100;not null;uniqueIndex:idx_auth_identity;comment:用户名/手机号/openid" json:"identifier"`                 // 认证标识
	Credential   string `gorm:"size:255;comment:bcrypt密码/OAuth token" json:"-"`                                                           // 认证凭证（不序列化）
}

func (UserAuth) TableName() string { return "user_auths" }
