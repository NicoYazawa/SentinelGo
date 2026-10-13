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
	"fmt"
	"log/slog"
	"os"

	"github.com/go-viper/mapstructure/v2"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/knadh/koanf/v2"
)

// backupCachePath 是主配置解析成功后写入的本地备份缓存路径。
// 当 bootstrap.yaml 损坏或不可读时，系统降级从此文件启动。
// 使用 var 而非 const 以便于测试时将备份重定向到临时目录，避免污染仓库。
var backupCachePath = "config/backup_cache.yaml"

// unmarshalConf 是 koanf 反序列化到 AppConfig 的统一配置。
//   - TagName: 使用 mapstructure tag（与结构体字段对齐）
//   - WeaklyTypedInput: 对齐 viper 行为，允许 int 等弱类型直接赋值到 time.Duration 等字段
//   - DecodeHook: 内置时间/切片解析 + 自动解密 ENC(...) 加密字段
var unmarshalConf = koanf.UnmarshalConf{
	Tag: "mapstructure",
	DecoderConfig: &mapstructure.DecoderConfig{
		TagName:          "mapstructure",
		WeaklyTypedInput: true,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.StringToSliceHookFunc(","),
			DecryptHook(),
		),
	},
}

// InitConfig 加载并解析 bootstrap.yaml，建立热更新监听。
//
// 启动流程：
//  1. 使用 koanf + file.Provider 读取 bootstrap.yaml
//  2. 通过 DecryptHook 自动解密 ENC() 加密字段
//  3. 反序列化为 AppConfig 并写入 GlobalManager
//  4. 成功后写入 backup_cache.yaml 作为容灾备份
//  5. 启动 file.Provider 监听，文件变更时自动热加载
//
// 容灾策略：
//   - 主配置读取失败 → 尝试从 backup_cache.yaml 降级启动
//   - 备份配置也不可用 → panic 拒绝启动（fail-fast）
func InitConfig(bootstrapPath string) {
	k := koanf.New(".")

	// 第一步：读取主配置文件
	fp := file.Provider(bootstrapPath)
	if err := k.Load(fp, yaml.Parser()); err != nil {
		slog.Warn("读取 bootstrap.yaml 失败，尝试备份缓存启动...", "error", err)
		loadFromBackup()
		return
	}

	// 注入环境变量覆盖（容器化部署用，前缀 KS_，优先级高于文件配置）。
	applyEnvOverrides(k)

	// 第二步：反序列化配置，同时通过 DecryptHook 解密敏感字段
	if err := unmarshal(k); err != nil {
		slog.Warn("解析配置失败，尝试备份缓存启动...", "error", err)
		loadFromBackup()
		return
	}

	// 第三步：写入本地备份缓存，供下次冷启动容灾
	writeBackup(bootstrapPath)

	// 第四步：启动文件变更监听，实现热更新（零停机）
	if err := fp.Watch(func(event any, err error) {
		if err != nil {
			slog.Error("配置文件监听错误", "error", err)
			return
		}
		slog.Info("检测到配置文件变更，开始热加载...")

		kk := koanf.New(".")
		if err := kk.Load(file.Provider(bootstrapPath), yaml.Parser()); err != nil {
			slog.Error("热加载读取配置失败，继续使用旧配置", "error", err)
			return
		}

		// 原子替换全局配置，并记录关键开关变更
		applyEnvOverrides(kk)
		var newCfg AppConfig
		if err := kk.UnmarshalWithConf("", &newCfg, unmarshalConf); err != nil {
			slog.Error("热加载配置失败，继续使用旧配置", "error", err)
			return
		}
		if err := validateConfig(&newCfg); err != nil {
			slog.Error("热加载认证配置无效，继续使用旧配置", "error", err)
			return
		}

		// 关键开关变更时发出警告，便于运维追溯
		oldCfg := GlobalManager.Get()
		if oldCfg.SystemSwitches.MaintenanceMode != newCfg.SystemSwitches.MaintenanceMode {
			slog.Warn("系统维护模式开关变更",
				"old", oldCfg.SystemSwitches.MaintenanceMode,
				"new", newCfg.SystemSwitches.MaintenanceMode,
			)
		}

		GlobalManager.Update(&newCfg)
		writeBackup(bootstrapPath)
		slog.Info("配置热加载完成")
	}); err != nil {
		slog.Warn("启动配置文件监听失败，热更新将不可用", "error", err)
	}
}

// applyEnvOverrides 通过环境变量覆盖 bootstrap.yaml 中的部分配置，
// 主要用于容器化部署（docker-compose）将 mysql/redis 等服务名注入连接串。
// 仅当对应环境变量非空时才覆盖，未设置时完全沿用文件配置（本地开发不受影响）。
//
//	支持（前缀 KS_）：
//	  KS_JWT_SECRET / KS_APP_NAME / KS_APP_ENV / KS_APP_PORT / KS_LOG_LEVEL
//	  KS_DATABASE_MASTER_DSN / KS_REDIS_ADDR / KS_REDIS_PASSWORD / KS_REDIS_DB
//	  KS_REDIS_POOL_SIZE / KS_REDIS_MIN_IDLE_CONNS
//	  KS_OAUTH_GITHUB_CLIENT_ID / KS_OAUTH_GITHUB_CLIENT_SECRET
//	  KS_OAUTH_GITHUB_REDIRECT_URI / KS_OAUTH_GITHUB_FRONTEND_URL
//	  KS_SMS_PROVIDER / KS_SMS_SIGN_NAME / KS_SMS_TEMPLATE_CODE
func applyEnvOverrides(k *koanf.Koanf) {
	overrides := map[string]string{
		"jwt.secret":                 os.Getenv("KS_JWT_SECRET"),
		"app.name":                   os.Getenv("KS_APP_NAME"),
		"app.env":                    os.Getenv("KS_APP_ENV"),
		"app.port":                   os.Getenv("KS_APP_PORT"),
		"log.level":                  os.Getenv("KS_LOG_LEVEL"),
		"database.master_dsn":        os.Getenv("KS_DATABASE_MASTER_DSN"),
		"redis.addr":                 os.Getenv("KS_REDIS_ADDR"),
		"redis.password":             os.Getenv("KS_REDIS_PASSWORD"),
		"redis.db":                   os.Getenv("KS_REDIS_DB"),
		"redis.pool_size":            os.Getenv("KS_REDIS_POOL_SIZE"),
		"redis.min_idle_conns":       os.Getenv("KS_REDIS_MIN_IDLE_CONNS"),
		"oauth.github.client_id":     os.Getenv("KS_OAUTH_GITHUB_CLIENT_ID"),
		"oauth.github.client_secret": os.Getenv("KS_OAUTH_GITHUB_CLIENT_SECRET"),
		"oauth.github.redirect_uri":  os.Getenv("KS_OAUTH_GITHUB_REDIRECT_URI"),
		"oauth.github.frontend_url":  os.Getenv("KS_OAUTH_GITHUB_FRONTEND_URL"),
		"sms.provider":               os.Getenv("KS_SMS_PROVIDER"),
		"sms.sign_name":              os.Getenv("KS_SMS_SIGN_NAME"),
		"sms.template_code":          os.Getenv("KS_SMS_TEMPLATE_CODE"),
	}
	for key, val := range overrides {
		if val != "" {
			k.Set(key, val)
		}
	}
}

// unmarshal 将 koanf 实例反序列化为 AppConfig 并写入全局配置管理器。
func unmarshal(k *koanf.Koanf) error {
	var cfg AppConfig
	if err := k.UnmarshalWithConf("", &cfg, unmarshalConf); err != nil {
		return err
	}
	if err := validateConfig(&cfg); err != nil {
		return err
	}
	GlobalManager.Update(&cfg)
	return nil
}

// writeBackup 将源配置文件原文写入本地备份缓存，供冷启动容灾。
func writeBackup(srcPath string) {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		slog.Warn("读取配置失败，无法写入备份缓存", "error", err)
		return
	}
	if err := os.WriteFile(backupCachePath, data, 0o644); err != nil {
		slog.Warn("写入配置备份缓存失败", "error", err)
	} else {
		slog.Info("配置已同步，本地备份缓存已更新", "path", backupCachePath)
	}
}

// loadFromBackup 从本地备份缓存降级启动。
// 如果备份也不可用，直接 panic 拒绝启动（fail-fast 策略）。
func loadFromBackup() {
	data, err := os.ReadFile(backupCachePath)
	if err != nil {
		panic(fmt.Sprintf("灾难：本地备份配置缓存不可用，拒绝启动！%v", err))
	}

	k := koanf.New(".")
	if err := k.Load(rawbytes.Provider(data), yaml.Parser()); err != nil {
		panic(fmt.Sprintf("灾难：解析备份配置失败，拒绝启动！%v", err))
	}
	applyEnvOverrides(k)

	var cfg AppConfig
	if err := k.UnmarshalWithConf("", &cfg, unmarshalConf); err != nil {
		panic(fmt.Sprintf("灾难：解析备份配置失败，拒绝启动！%v", err))
	}
	if err := validateConfig(&cfg); err != nil {
		panic(err)
	}

	GlobalManager.Update(&cfg)
	slog.Warn("已通过本地备份缓存降级启动，部分功能可能受限")
}

func validateConfig(cfg *AppConfig) error {
	if len(cfg.JWT.Secret) < 32 {
		return fmt.Errorf("JWT 签名密钥至少需要 32 字节，请设置 KS_JWT_SECRET")
	}
	if cfg.App.Env != "dev" && cfg.SMS.Provider != "aliyun" {
		return fmt.Errorf("非开发环境必须配置真实短信通道")
	}
	if cfg.OAuth.GitHub.ClientID != "" && cfg.OAuth.GitHub.ClientSecret == "" {
		return fmt.Errorf("OAuth Client Secret 未配置")
	}
	return nil
}
