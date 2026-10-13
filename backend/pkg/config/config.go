// Package config 提供统一配置管理能力，支持热更新、备份容灾、敏感信息加密存储。
//
// 核心特性：
//   - koanf 驱动的 YAML 配置加载与反序列化
//   - file.Provider 文件变更监听，实现热更新零停机
//   - ENC() 加密占位符自动解密（AES-256-CTR），保护数据库密码等敏感信息
//   - 本地 backup_cache.yaml 备份缓存，主配置损坏时自动降级启动
//   - sync.RWMutex 保护的并发安全配置读写
package config

import (
	"sync"
	"time"
)

// AppConfig 是应用全局配置的顶层结构体，聚合了所有子系统配置。
// 字段通过 mapstructure tag 与 bootstrap.yaml 的 key 对齐。
type AppConfig struct {
	App            AppCfg         `mapstructure:"app"`             // 应用基础信息
	JWT            JWTCfg         `mapstructure:"jwt"`             // JWT 令牌配置
	Database       DatabaseCfg    `mapstructure:"database"`        // 数据库连接池与主从配置
	Redis          RedisCfg       `mapstructure:"redis"`           // Redis 连接与连接池配置
	Log            LogCfg         `mapstructure:"log"`             // 日志级别配置
	OAuth          OAuthCfg       `mapstructure:"oauth"`           // 第三方 OAuth 登录配置
	SMS            SMSCfg         `mapstructure:"sms"`             // 短信验证码服务配置
	SystemSwitches SystemSwitches `mapstructure:"system_switches"` // 系统功能开关
}

// SystemSwitches 包含可在运行时通过热更新切换的功能开关。
type SystemSwitches struct {
	EnableRegister  bool `mapstructure:"enable_register"`  // 是否开放新用户注册
	MaintenanceMode bool `mapstructure:"maintenance_mode"` // 是否开启维护模式（开启后所有请求返回维护提示）
}

// AppCfg 定义应用基础元信息。
type AppCfg struct {
	Name string `mapstructure:"name"` // 应用名称，用于日志标识与服务注册
	Env  string `mapstructure:"env"`  // 运行环境：dev / test / prod
	Port int    `mapstructure:"port"` // HTTP 监听端口
}

// JWTCfg 定义 JSON Web Token 的签名与有效期配置。
type JWTCfg struct {
	Secret string        `mapstructure:"secret"` // JWT 签名密钥（HMAC-SHA256）
	Expire time.Duration `mapstructure:"expire"` // Access Token 过期时间
}

// DatabaseCfg 定义 MySQL 数据库连接池与读写分离配置。
type DatabaseCfg struct {
	MasterDSN    string        `mapstructure:"master_dsn"`     // 主库（写库）连接字符串
	SlaveDSNs    []string      `mapstructure:"slave_dsns"`     // 从库（读库）连接字符串列表，为空时不启用读写分离
	MaxOpenConns int           `mapstructure:"max_open_conns"` // 连接池最大打开连接数
	MaxIdleConns int           `mapstructure:"max_idle_conns"` // 连接池最大空闲连接数
	MaxLifetime  time.Duration `mapstructure:"max_lifetime"`   // 单个连接的最大存活时间
	MaxIdleTime  time.Duration `mapstructure:"max_idle_time"`  // 空闲连接的最大保持时间
}

// RedisCfg 定义 Redis 连接与连接池配置。
type RedisCfg struct {
	Addr         string `mapstructure:"addr"`           // Redis 地址，格式 host:port
	Password     string `mapstructure:"password"`       // Redis 认证密码
	DB           int    `mapstructure:"db"`             // Redis 数据库编号（0-15）
	PoolSize     int    `mapstructure:"pool_size"`      // 连接池最大连接数
	MinIdleConns int    `mapstructure:"min_idle_conns"` // 连接池最小空闲连接数
}

// LogCfg 定义日志输出级别。
type LogCfg struct {
	Level string `mapstructure:"level"` // 日志级别：debug / info / warn / error
}

// OAuthCfg 聚合所有第三方 OAuth 提供商的配置。
type OAuthCfg struct {
	GitHub GitHubCfg `mapstructure:"github"` // GitHub OAuth App 配置
}

// GitHubCfg 定义 GitHub OAuth App 的客户端参数。
type GitHubCfg struct {
	ClientID     string `mapstructure:"client_id"`     // GitHub OAuth App Client ID
	ClientSecret string `mapstructure:"client_secret"` // GitHub OAuth App Client Secret（支持 ENC() 加密）
	RedirectURI  string `mapstructure:"redirect_uri"`  // OAuth 回调地址
	FrontendURL  string `mapstructure:"frontend_url"`  // 前端地址，用于 OAuth 成功后重定向
	HTTPProxy    string `mapstructure:"http_proxy"`    // HTTP 代理地址，用于内网环境访问 GitHub API
}

// SMSCfg 定义短信验证码服务的运行配置。
// 阿里云 AK/SK 不在此处落明文，由 credentials-go 默认凭据链从环境变量读取。
type SMSCfg struct {
	Provider     string        `mapstructure:"provider"`      // 下发通道：dev（仅日志）| aliyun（真实网关）
	SignName     string        `mapstructure:"sign_name"`     // 短信签名，如「速通互联验证码」
	TemplateCode string        `mapstructure:"template_code"` // 短信模板 ID，如「100001」
	CodeTTL      time.Duration `mapstructure:"code_ttl"`      // 验证码有效期，默认 5m
	RateLimit    time.Duration `mapstructure:"rate_limit"`    // 发送频率限制，默认 60s
	MaxAttempts  int           `mapstructure:"max_attempts"`  // 单码最大校验次数，默认 5
}

// ConfigManager 提供并发安全的配置访问与更新能力。
// 使用 sync.RWMutex 保护，支持多读单写。
type ConfigManager struct {
	mu     sync.RWMutex // 读写锁，保护 Active 指针的并发访问
	Active *AppConfig   // 当前生效的配置实例
}

// GlobalManager 是应用全局唯一的配置管理器实例。
// 启动时由 InitConfig 填充，运行期间可通过热更新替换。
var GlobalManager = &ConfigManager{Active: &AppConfig{}}

// Get 返回当前生效的配置（读锁保护，并发安全）。
func (m *ConfigManager) Get() *AppConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.Active
}

// Update 原子性地替换当前配置（写锁保护，并发安全）。
// 通常在热更新或备份降级启动时调用。
func (m *ConfigManager) Update(newCfg *AppConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Active = newCfg
}
