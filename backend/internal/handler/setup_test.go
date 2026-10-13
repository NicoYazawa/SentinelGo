package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"sentinelgo/internal/model"
	"sentinelgo/internal/repository"
	"sentinelgo/internal/service"
	"sentinelgo/pkg/casbin"
	"sentinelgo/pkg/config"
	myoauth "sentinelgo/pkg/oauth"
	"sentinelgo/pkg/sms"
	"sentinelgo/pkg/token"
	"sentinelgo/pkg/utils"
)

func setupHandler(t *testing.T) (*gorm.DB, *service.AuthService, *gin.Engine) {
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
	service.InitMultiAuth(
		sms.NewSMSService(nil, sms.NewDevSender(), sms.Config{}),
		myoauth.NewGitHubOAuth("test", "test", "http://localhost/callback", "http://localhost:3001", ""),
	)

	for _, r := range []string{"admin", "user"} {
		db.Create(&model.Role{Code: r, Name: r, Status: 1})
	}

	ur := repository.NewUserRepository(db)
	rr := repository.NewRoleRepository(db)
	authSvc := service.NewAuthService(ur, rr, db)
	return db, authSvc, gin.New()
}

func hctx(t *testing.T, method, path string, body any, userID uint64, sessionID string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, &buf)
	c.Request.Header.Set("Content-Type", "application/json")
	if userID != 0 {
		c.Set("current_user_id", userID)
	}
	if sessionID != "" {
		c.Set("current_session_id", sessionID)
	}
	// 直接调用 handler（不经过路由匹配），需手动把路径中的数字段作为 :id 路由参数注入
	setIDParam(c, path)
	return c, w
}

// setIDParam 从请求路径中提取第一个全数字段作为 :id 路由参数，
// 使得在测试中直接调用带 c.Param("id") 的 handler 方法也能拿到正确的 ID。
func setIDParam(c *gin.Context, path string) {
	if i := strings.Index(path, "?"); i >= 0 {
		path = path[:i]
	}
	for _, seg := range strings.Split(path, "/") {
		if seg != "" {
			allDigits := true
			for _, ch := range seg {
				if ch < '0' || ch > '9' {
					allDigits = false
					break
				}
			}
			if allDigits {
				c.Params = gin.Params{{Key: "id", Value: seg}}
				return
			}
		}
	}
}

func mustCreateUser(t *testing.T, db *gorm.DB, username, pwd string, status int8) model.User {
	t.Helper()
	hash, _ := utils.HashPassword(pwd)
	u := model.User{Username: username, Password: hash, Locale: "zh-CN"}
	if err := db.Create(&u).Error; err != nil {
		t.Fatal(err)
	}
	if status != 1 {
		db.Model(&model.User{}).Where("id = ?", u.ID).Update("status", status)
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
