package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/knadh/koanf/v2"
)

func TestApplyEnvOverrides(t *testing.T) {
	t.Setenv("KS_JWT_SECRET", "env-secret-value")
	t.Setenv("KS_REDIS_ADDR", "redis:6379")
	t.Setenv("KS_APP_PORT", "9090")
	t.Setenv("KS_SMS_PROVIDER", "aliyun")
	// 空值不应覆盖原配置
	t.Setenv("KS_APP_NAME", "")

	k := koanf.New(".")
	k.Set("app.name", "origin")
	applyEnvOverrides(k)

	if got := k.String("jwt.secret"); got != "env-secret-value" {
		t.Errorf("jwt.secret 覆盖失败: %q", got)
	}
	if got := k.String("redis.addr"); got != "redis:6379" {
		t.Errorf("redis.addr 覆盖失败: %q", got)
	}
	if got := k.String("app.port"); got != "9090" {
		t.Errorf("app.port 覆盖失败: %q", got)
	}
	if got := k.String("sms.provider"); got != "aliyun" {
		t.Errorf("sms.provider 覆盖失败: %q", got)
	}
	// 空环境变量不改变原值
	if got := k.String("app.name"); got != "origin" {
		t.Errorf("空 env 不应覆盖原值: %q", got)
	}
}

func TestUnmarshalHappy(t *testing.T) {
	k := koanf.New(".")
	k.Set("app.env", "dev")
	k.Set("jwt.secret", "0123456789abcdef0123456789abcdef")
	k.Set("sms.provider", "dev")
	if err := unmarshal(k); err != nil {
		t.Fatal(err)
	}
	if got := GlobalManager.Get().JWT.Secret; got != "0123456789abcdef0123456789abcdef" {
		t.Errorf("GlobalManager 未正确更新: %q", got)
	}
}

func TestValidateConfigPositive(t *testing.T) {
	cfg := &AppConfig{
		App: AppCfg{Env: "dev"},
		JWT: JWTCfg{Secret: "0123456789abcdef0123456789abcdef"},
		SMS: SMSCfg{Provider: "dev"},
	}
	if err := validateConfig(cfg); err != nil {
		t.Errorf("合法配置不应报错: %v", err)
	}
}

func TestWriteAndLoadBackup(t *testing.T) {
	dir := t.TempDir()
	old := backupCachePath
	backupCachePath = filepath.Join(dir, "backup_cache.yaml")
	defer func() { backupCachePath = old }()

	src := filepath.Join(dir, "bootstrap.yaml")
	content := []byte("app:\n  env: dev\n  name: backup-test\njwt:\n  secret: 0123456789abcdef0123456789abcdef\nsms:\n  provider: dev\n")
	if err := os.WriteFile(src, content, 0o644); err != nil {
		t.Fatal(err)
	}

	writeBackup(src)
	if _, err := os.Stat(backupCachePath); err != nil {
		t.Fatalf("备份文件未写入: %v", err)
	}

	// 清空当前配置后从备份恢复
	GlobalManager.Update(&AppConfig{})
	loadFromBackup()
	if got := GlobalManager.Get().App.Name; got != "backup-test" {
		t.Errorf("从备份恢复失败: %q", got)
	}
}

func TestLoadFromBackupPanicsWithoutFile(t *testing.T) {
	dir := t.TempDir()
	old := backupCachePath
	backupCachePath = filepath.Join(dir, "missing_backup.yaml")
	defer func() { backupCachePath = old }()

	defer func() {
		if r := recover(); r == nil {
			t.Error("缺少备份文件时应 panic")
		}
	}()
	loadFromBackup()
}

func TestInitConfigSuccess(t *testing.T) {
	dir := t.TempDir()
	old := backupCachePath
	backupCachePath = filepath.Join(dir, "backup_cache.yaml")
	defer func() { backupCachePath = old }()

	path := filepath.Join(dir, "bootstrap.yaml")
	content := []byte("app:\n  env: dev\n  name: init-test\n  port: 8080\njwt:\n  secret: 0123456789abcdef0123456789abcdef\nsms:\n  provider: dev\nredis:\n  addr: 127.0.0.1:6379\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	InitConfig(path)
	if got := GlobalManager.Get().App.Name; got != "init-test" {
		t.Errorf("InitConfig 后 App.Name 未更新: %q", got)
	}
	if _, err := os.Stat(backupCachePath); err != nil {
		t.Errorf("InitConfig 成功应写入备份缓存: %v", err)
	}
}

func TestInitConfigFallbackPanics(t *testing.T) {
	dir := t.TempDir()
	old := backupCachePath
	backupCachePath = filepath.Join(dir, "missing_backup.yaml")
	defer func() { backupCachePath = old }()

	defer func() {
		if r := recover(); r == nil {
			t.Error("主配置缺失且无备份时应 panic")
		}
	}()
	InitConfig(filepath.Join(dir, "nonexistent.yaml"))
}

func TestInitConfigUnmarshalErrorFallsBack(t *testing.T) {
	dir := t.TempDir()
	old := backupCachePath
	backupCachePath = filepath.Join(dir, "backup_cache.yaml")
	defer func() { backupCachePath = old }()

	// 先写入一份合法备份，供解析失败时回退
	backupSrc := filepath.Join(dir, "bootstrap_backup.yaml")
	backupContent := []byte("app:\n  env: dev\n  name: fallback-test\njwt:\n  secret: 0123456789abcdef0123456789abcdef\nsms:\n  provider: dev\n")
	if err := os.WriteFile(backupSrc, backupContent, 0o644); err != nil {
		t.Fatal(err)
	}
	writeBackup(backupSrc)

	// 主配置 JWT 密钥过短，解析/校验会失败，触发回退备份
	badPath := filepath.Join(dir, "bad_bootstrap.yaml")
	badContent := []byte("app:\n  env: dev\njwt:\n  secret: short\nsms:\n  provider: dev\n")
	if err := os.WriteFile(badPath, badContent, 0o644); err != nil {
		t.Fatal(err)
	}

	InitConfig(badPath)
	if got := GlobalManager.Get().App.Name; got != "fallback-test" {
		t.Errorf("解析失败应回退到备份配置: %q", got)
	}
}
