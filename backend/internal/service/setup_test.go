package service

import (
	"io"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"sentinelgo/internal/model"
	"sentinelgo/internal/repository"
	"sentinelgo/pkg/casbin"
	"sentinelgo/pkg/config"
	myoauth "sentinelgo/pkg/oauth"
	"sentinelgo/pkg/sms"
	"sentinelgo/pkg/token"
	"sentinelgo/pkg/utils"
)

func setupService(t *testing.T) (*gorm.DB, *AuthService) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	gin.DefaultWriter = io.Discard

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })

	if err := db.AutoMigrate(
		&model.User{}, &model.Role{}, &model.Menu{}, &model.Permission{},
		&model.CasbinRule{}, &model.OperationLog{}, &model.UserRole{},
		&model.RoleMenu{}, &model.UserAuth{}, &model.AuthSession{}, &model.OAuthState{},
	); err != nil {
		t.Fatal(err)
	}

	casbin.Init(db, "../../config/rbac_model.conf")
	config.GlobalManager.Update(&config.AppConfig{
		App:            config.AppCfg{Env: "dev"},
		SMS:            config.SMSCfg{Provider: "dev"},
		SystemSwitches: config.SystemSwitches{EnableRegister: true},
	})
	token.Init(strings.Repeat("test-key-", 5), nil)
	InitMultiAuth(
		sms.NewSMSService(nil, sms.NewDevSender(), sms.Config{}),
		myoauth.NewGitHubOAuth("test", "test", "http://localhost/callback", "http://localhost:3001", ""),
	)

	for _, r := range []string{"admin", "user"} {
		db.Create(&model.Role{Code: r, Name: r, Status: 1})
	}
	casbin.AddPolicy("admin", "*", "*")
	casbin.AddPolicy("user", "/api/v1/user/info", "GET")

	ur := repository.NewUserRepository(db)
	rr := repository.NewRoleRepository(db)
	return db, NewAuthService(ur, rr, db)
}

func mustCreateUser(t *testing.T, db *gorm.DB, username, pwd string, status int8) model.User {
	t.Helper()
	hash, _ := utils.HashPassword(pwd)
	u := model.User{Username: username, Password: hash, Locale: "zh-CN"}
	if err := db.Create(&u).Error; err != nil {
		t.Fatal(err)
	}
	// GORM skips zero-value Status on Create, so set it explicitly.
	if status != 1 {
		if err := db.Model(&model.User{}).Where("id = ?", u.ID).Update("status", status).Error; err != nil {
			t.Fatal(err)
		}
		u.Status = status
	}
	return u
}

func mustAssignRole(t *testing.T, db *gorm.DB, userID uint64, code string) {
	t.Helper()
	var role model.Role
	if err := db.Where("code = ?", code).First(&role).Error; err != nil {
		t.Fatal(err)
	}
	repo := repository.NewUserRepository(db)
	if err := repo.AssignRoles(userID, []uint64{role.ID}); err != nil {
		t.Fatal(err)
	}
}
