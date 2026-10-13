package main

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"sentinelgo/internal/model"
	"sentinelgo/pkg/casbin"
)

// covDB 构建隔离的 sqlite 内存库并复用 main 的迁移与 Casbin 初始化。
func covDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })
	// InitAdmin 要求设置首次启动管理员密码（≥12 字节）
	t.Setenv("KS_BOOTSTRAP_ADMIN_PASSWORD", "BootstrapPassword123!")
	runMigrations(db)
	casbin.Init(db, "../../config/rbac_model.conf")
	return db
}

func TestInitAdminUserWrapper(t *testing.T) {
	db := covDB(t)
	initAdminUser(db)
	var u model.User
	if err := db.Where("username = ?", "admin").First(&u).Error; err != nil {
		t.Fatalf("admin 未创建: %v", err)
	}
	// 幂等：再次调用不应报错
	initAdminUser(db)
}

func TestInitDefaultMenusWrapper(t *testing.T) {
	db := covDB(t)
	initAdminUser(db) // 创建 admin 角色（ID=1），供菜单分配
	initDefaultMenus(db)
	var cnt int64
	db.Model(&model.Menu{}).Count(&cnt)
	if cnt == 0 {
		t.Error("默认菜单未创建")
	}
	// 幂等
	initDefaultMenus(db)
}

func TestInitDefaultPermissionsWrapper(t *testing.T) {
	db := covDB(t)
	initDefaultPermissions(db)
	var cnt int64
	db.Model(&model.Permission{}).Count(&cnt)
	if cnt == 0 {
		t.Error("默认权限未创建")
	}
	// 幂等
	initDefaultPermissions(db)
}
