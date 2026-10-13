package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"sentinelgo/internal/model"
	"sentinelgo/internal/repository"
	"sentinelgo/pkg/casbin"
	"sentinelgo/pkg/response"
	"sentinelgo/pkg/token"
)

func TestAuditCapturesBusinessFailureAndIdentity(t *testing.T) {
	db := newMWDB(t)
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	r := gin.New()
	r.Use(NewAuditMiddleware(repository.NewOperationLogRepository(db)))
	r.POST("/api/users", func(c *gin.Context) {
		c.Set("current_user_id", uint64(42))
		c.Set("current_username", "alice")
		c.Set("trace_id", "trace-test")
		c.JSON(200, gin.H{"code": 11000, "msg": "invalid update"})
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/users", nil)
	req.Header.Set("User-Agent", "audit-test")
	r.ServeHTTP(w, req)
	if w.Body.String() != `{"code":11000,"msg":"invalid update"}` {
		t.Fatalf("response changed: %s", w.Body.String())
	}
	deadline := time.Now().Add(time.Second)
	for {
		var logs []model.OperationLog
		if err := db.Find(&logs).Error; err != nil {
			t.Fatal(err)
		}
		if len(logs) == 1 {
			l := logs[0]
			if l.Status != 0 || l.ErrorMsg != "invalid update" || l.UserID != 42 || l.Username != "alice" || l.TraceID != "trace-test" || l.UserAgent != "audit-test" {
				t.Fatalf("incorrect audit record: %+v", l)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("audit record not persisted")
		}
		time.Sleep(time.Millisecond * 5)
	}
}
func TestJWTRejectsSessionAccountAndPermissionFailures(t *testing.T) {
	for _, scenario := range []string{"expired session", "disabled account", "missing enforcer", "roles query error", "bootstrap", "forbidden"} {
		t.Run(scenario, func(t *testing.T) {
			db := newMWDB(t)
			sqlDB, _ := db.DB()
			defer sqlDB.Close()
			casbin.Init(db, "../../config/rbac_model.conf")
			token.Init("test-secret-key-1234567890abcdef", nil)
			u := model.User{Username: "branch", Status: 1, Locale: "en-US"}
			if err := db.Create(&u).Error; err != nil {
				t.Fatal(err)
			}
			sess := model.AuthSession{ID: "branch-session", UserID: u.ID, RefreshJTI: "jti", ExpiresAt: time.Now().Add(time.Hour)}
			if err := db.Create(&sess).Error; err != nil {
				t.Fatal(err)
			}
			want, msg := response.ErrUnauthorized, "登录会话已失效"
			switch scenario {
			case "expired session":
				db.Model(&sess).Update("expires_at", time.Now().Add(-time.Hour))
			case "disabled account":
				db.Model(&u).Update("status", 0)
				msg = "账户不可用，请重新登录"
			case "missing enforcer":
				old := casbin.Enforcer
				casbin.Enforcer = nil
				defer func() { casbin.Enforcer = old }()
				want, msg = response.ErrSystemError, "权限服务不可用"
			case "roles query error":
				db.Callback().Query().Before("gorm:query").Register("fail_roles", func(tx *gorm.DB) {
					if tx.Statement.Table == "sys_role" {
						tx.AddError(errors.New("roles unavailable"))
					}
				})
				want, msg = response.ErrSystemError, "无法验证权限"
			case "bootstrap":
				db.Model(&u).Update("password_change_required", true)
				want, msg = response.ErrPasswordChangeRequired, "请先设置正式密码"
			case "forbidden":
				want, msg = response.ErrForbidden, "权限不足，拒绝访问"
			}
			access, err := token.GenerateAccessToken(u.ID, u.Username, nil, "en-US", sess.ID)
			if err != nil {
				t.Fatal(err)
			}
			r := gin.New()
			r.Use(JWTAuth(db))
			ran := false
			r.GET("/api/protected", func(c *gin.Context) { ran = true; c.Status(204) })
			req := httptest.NewRequest("GET", "/api/protected", nil)
			req.Header.Set("Authorization", "Bearer "+access)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			var got response.Response
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if ran || got.Code != want || got.Msg != msg {
				t.Fatalf("ran=%v response=%+v want=%d %s", ran, got, want, msg)
			}
		})
	}
}
func TestJWTAllowsLogoutDuringBootstrap(t *testing.T) {
	db := newMWDB(t)
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	token.Init("test-secret-key-1234567890abcdef", nil)
	u := model.User{Username: "bootstrap", Status: 1, PasswordChangeRequired: true}
	db.Create(&u)
	db.Create(&model.AuthSession{ID: "bootstrap-session", UserID: u.ID, ExpiresAt: time.Now().Add(time.Hour)})
	access, e := token.GenerateAccessToken(u.ID, u.Username, nil, "", "bootstrap-session")
	if e != nil {
		t.Fatal(e)
	}
	r := gin.New()
	r.Use(JWTAuth(db))
	r.POST("/api/v1/auth/logout", func(c *gin.Context) {
		if c.GetUint64("current_user_id") != u.ID {
			t.Fatal("identity missing")
		}
		c.Status(http.StatusNoContent)
	})
	req := httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 204 {
		t.Fatalf("logout blocked: %s", w.Body.String())
	}
}
func TestTraceWithoutMiddleware(t *testing.T) {
	if GetTraceID(context.Background()) != "" {
		t.Fatal("unrelated context has trace")
	}
}
