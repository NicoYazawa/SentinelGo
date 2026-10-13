package config

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/knadh/koanf/v2"
)

type reloadLog struct{ messages chan string }

func (h reloadLog) Enabled(context.Context, slog.Level) bool { return true }
func (h reloadLog) Handle(_ context.Context, r slog.Record) error {
	select {
	case h.messages <- r.Message:
	default:
	}
	return nil
}
func (h reloadLog) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h reloadLog) WithGroup(string) slog.Handler      { return h }
func awaitReload(t *testing.T, messages <-chan string, want string) {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case got := <-messages:
			if got == want {
				return
			}
		case <-deadline.C:
			t.Fatalf("configuration watcher did not report %q", want)
		}
	}
}
func TestConfigurationHotReloadAndRejectedUpdates(t *testing.T) {
	dir := t.TempDir()
	oldBackup, oldConfig, oldLogger := backupCachePath, GlobalManager.Get(), slog.Default()
	backupCachePath = filepath.Join(dir, "backup.yaml")
	t.Cleanup(func() { backupCachePath = oldBackup; GlobalManager.Update(oldConfig); slog.SetDefault(oldLogger) })
	messages := make(chan string, 100)
	slog.SetDefault(slog.New(reloadLog{messages}))
	path := filepath.Join(dir, "bootstrap.yaml")
	valid := func(name string, maintenance bool) string {
		return fmt.Sprintf("app:\n  env: dev\n  name: %s\njwt:\n  secret: %s\nsms:\n  provider: dev\nsystem_switches:\n  maintenance_mode: %t\n", name, strings.Repeat("k", 32), maintenance)
	}
	if err := os.WriteFile(path, []byte(valid("original", false)), 0600); err != nil {
		t.Fatal(err)
	}
	InitConfig(path)
	if GlobalManager.Get().App.Name != "original" {
		t.Fatal("initial config not loaded")
	}
	// The file provider drops identical write events within its 5ms window.
	// Keep each independent update outside that window, including on fast Linux CI.
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(path, []byte(valid("reloaded", true)), 0600); err != nil {
		t.Fatal(err)
	}
	awaitReload(t, messages, "配置热加载完成")
	if cfg := GlobalManager.Get(); cfg.App.Name != "reloaded" || !cfg.SystemSwitches.MaintenanceMode {
		t.Fatalf("reload failed: %+v", cfg)
	}
	backup, err := os.ReadFile(backupCachePath)
	if err != nil || !strings.Contains(string(backup), "reloaded") {
		t.Fatalf("reload not backed up: %v", err)
	}
	for _, tc := range []struct{ name, text, log string }{
		{"invalid YAML", "app: [", "热加载读取配置失败，继续使用旧配置"},
		{"invalid field type", "app:\n  port: invalid-port\n", "热加载配置失败，继续使用旧配置"},
		{"unsafe key", "jwt:\n  secret: short\n", "热加载认证配置无效，继续使用旧配置"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			time.Sleep(10 * time.Millisecond)
			if err := os.WriteFile(path, []byte(tc.text), 0600); err != nil {
				t.Fatal(err)
			}
			awaitReload(t, messages, tc.log)
			if GlobalManager.Get().App.Name != "reloaded" {
				t.Fatal("invalid reload replaced active config")
			}
			got, err := os.ReadFile(backupCachePath)
			if err != nil || string(got) != string(backup) {
				t.Fatal("invalid reload overwrote usable backup")
			}
		})
	}
}

func TestBackupFailureRetainsActiveConfiguration(t *testing.T) {
	dir := t.TempDir()
	old := backupCachePath
	backupCachePath = filepath.Join(dir, "existing.yaml")
	t.Cleanup(func() { backupCachePath = old })
	if err := os.WriteFile(backupCachePath, []byte("preserved"), 0600); err != nil {
		t.Fatal(err)
	}
	writeBackup(filepath.Join(dir, "missing.yaml"))
	got, err := os.ReadFile(backupCachePath)
	if err != nil || string(got) != "preserved" {
		t.Fatal("missing source damaged backup")
	}
	backupCachePath = dir
	writeBackup(filepath.Join(dir, "existing.yaml"))
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Fatal("failed backup damaged destination directory")
	}
}

func TestMalformedBackupRejectsStartup(t *testing.T) {
	for _, text := range []string{"app: [", "app:\n  port: invalid\n", "jwt:\n  secret: short\n"} {
		t.Run(text, func(t *testing.T) {
			oldBackup, oldConfig := backupCachePath, GlobalManager.Get()
			backupCachePath = filepath.Join(t.TempDir(), "backup.yaml")
			t.Cleanup(func() { backupCachePath = oldBackup; GlobalManager.Update(oldConfig) })
			if err := os.WriteFile(backupCachePath, []byte(text), 0600); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if recover() == nil {
					t.Error("invalid backup allowed startup")
				}
				if GlobalManager.Get() != oldConfig {
					t.Error("invalid backup replaced active configuration")
				}
			}()
			loadFromBackup()
		})
	}
}

func TestUnmarshalInvalidFieldPreservesActiveConfig(t *testing.T) {
	old := GlobalManager.Get()
	k := koanf.New(".")
	if err := k.Set("app.port", "invalid"); err != nil {
		t.Fatal(err)
	}
	if err := unmarshal(k); err == nil {
		t.Fatal("invalid port accepted")
	}
	if GlobalManager.Get() != old {
		t.Fatal("invalid configuration replaced active pointer")
	}
}

func TestOAuthConfigRequiresClientSecret(t *testing.T) {
	cfg := &AppConfig{App: AppCfg{Env: "dev"}, JWT: JWTCfg{Secret: strings.Repeat("k", 32)}, SMS: SMSCfg{Provider: "dev"}, OAuth: OAuthCfg{GitHub: GitHubCfg{ClientID: "configured"}}}
	if err := validateConfig(cfg); err == nil || !strings.Contains(err.Error(), "Client Secret") {
		t.Fatalf("OAuth client without secret accepted: %v", err)
	}
}
