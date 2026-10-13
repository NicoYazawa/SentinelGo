package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"sentinelgo/internal/model"
	"sentinelgo/internal/repository"
	"sentinelgo/pkg/casbin"
	"sentinelgo/pkg/i18n"
	"sentinelgo/pkg/token"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestGin() (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	return c, w
}

func newMWDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.User{}, &model.Role{}, &model.AuthSession{}, &model.CasbinRule{}, &model.OperationLog{}, &model.UserRole{}, &model.UserAuth{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestI18n(t *testing.T) {
	c, w := newTestGin()
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.Header.Set(i18n.HeaderAcceptLanguage, "en-US")
	I18n()(c)
	if i18n.FromContext(c) != i18n.EnUS {
		t.Error("language not set from header")
	}
	if w.Header().Get("Content-Language") != "en-US" {
		t.Error("content-language not set")
	}
}

func TestCors(t *testing.T) {
	c, w := newTestGin()
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.Header.Set("Origin", "http://x.com")
	Cors()(c)
	if w.Header().Get("Access-Control-Allow-Origin") != "http://x.com" {
		t.Error("origin not reflected")
	}
	if c.IsAborted() {
		t.Error("normal request should not abort")
	}

	// OPTIONS preflight aborts with 204
	c2, w2 := newTestGin()
	c2.Request = httptest.NewRequest(http.MethodOptions, "/", nil)
	c2.Request.Header.Set("Origin", "http://x.com")
	Cors()(c2)
	if w2.Code != http.StatusNoContent {
		t.Errorf("preflight status = %d", w2.Code)
	}
}

func TestTrace(t *testing.T) {
	c, w := newTestGin()
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.Header.Set("X-Trace-Id", "inherited-123")
	Trace()(c)
	if c.GetString(TraceIDKey) != "inherited-123" {
		t.Error("trace id not inherited")
	}
	if w.Header().Get("X-Trace-Id") != "inherited-123" {
		t.Error("trace header not set")
	}
	if GetTraceID(c.Request.Context()) != "inherited-123" {
		t.Error("trace not in request context")
	}
}

func TestTraceGenerates(t *testing.T) {
	c, _ := newTestGin()
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	Trace()(c)
	if c.GetString(TraceIDKey) == "" {
		t.Error("trace id should be generated")
	}
}

func TestLogger(t *testing.T) {
	c, _ := newTestGin()
	c.Request = httptest.NewRequest(http.MethodGet, "/p", nil)
	Logger()(c)
	if c.IsAborted() {
		t.Error("logger should not abort")
	}
}

func TestMetrics(t *testing.T) {
	c, _ := newTestGin()
	c.Request = httptest.NewRequest(http.MethodGet, "/p", nil)
	Metrics()(c)
	if c.IsAborted() {
		t.Error("metrics should not abort")
	}
}

func TestRecoveryNoPanic(t *testing.T) {
	c, _ := newTestGin()
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	Recovery()(c)
	if c.IsAborted() {
		t.Error("no panic should not abort")
	}
}

func TestRecoveryPanic(t *testing.T) {
	w := httptest.NewRecorder()
	_, engine := gin.CreateTestContext(w)
	engine.Use(Recovery())
	engine.GET("/panic", func(c *gin.Context) { panic("boom") })
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if w.Body.Len() == 0 {
		t.Error("should have written response on panic")
	}
}

func TestRateLimitAllow(t *testing.T) {
	c, _ := newTestGin()
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.RemoteAddr = "1.2.3.4:8080"
	RateLimit()(c)
	if c.IsAborted() {
		t.Error("request should be allowed")
	}
}

func TestRateLimitDeny(t *testing.T) {
	old := GlobalLimiter
	GlobalLimiter = NewIPRateLimiter(0, 0)
	defer func() { GlobalLimiter = old }()

	c, w := newTestGin()
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.RemoteAddr = "9.9.9.9:8080"
	RateLimit()(c)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}
}

func TestCasbinCheckDeny(t *testing.T) {
	db := newMWDB(t)
	casbin.Init(db, "../../config/rbac_model.conf")
	InitCasbin(casbin.Enforcer)

	c, _ := newTestGin()
	c.Request = httptest.NewRequest(http.MethodGet, "/api/x", nil)
	CasbinCheck()(c)
	if !c.IsAborted() {
		t.Error("missing roles should be denied")
	}

	// with a role that has no policy -> denied
	c2, _ := newTestGin()
	c2.Request = httptest.NewRequest(http.MethodGet, "/api/x", nil)
	c2.Set("current_user_roles", []string{"user"})
	CasbinCheck()(c2)
	if !c2.IsAborted() {
		t.Error("no policy should be denied")
	}
}

func TestCasbinCheckAllowed(t *testing.T) {
	db := newMWDB(t)
	casbin.Init(db, "../../config/rbac_model.conf")
	InitCasbin(casbin.Enforcer)
	if _, err := casbin.AddPolicy("user", "/api/ok", "GET"); err != nil {
		t.Fatal(err)
	}
	c, _ := newTestGin()
	c.Request = httptest.NewRequest(http.MethodGet, "/api/ok", nil)
	c.Set("current_user_roles", []string{"user"})
	CasbinCheck()(c)
	if c.IsAborted() {
		t.Error("policy should allow")
	}
}

func TestJWTAuthDenyPaths(t *testing.T) {
	c, _ := newTestGin()
	c.Request = httptest.NewRequest(http.MethodGet, "/api/x", nil)
	JWTAuth(nil)(c)
	if !c.IsAborted() {
		t.Error("missing bearer should be denied")
	}

	c2, _ := newTestGin()
	c2.Request = httptest.NewRequest(http.MethodGet, "/api/x", nil)
	c2.Request.Header.Set("Authorization", "Bearer bad.token.here")
	JWTAuth(nil)(c2)
	if !c2.IsAborted() {
		t.Error("bad token should be denied")
	}
}

func TestJWTAuthSuccess(t *testing.T) {
	db := newMWDB(t)
	casbin.Init(db, "../../config/rbac_model.conf")
	token.Init("test-secret-key-1234567890abcdef", nil)
	role := &model.Role{Name: "user", Code: "user", Status: 1}
	db.Create(role)
	user := &model.User{Username: "jwtuser", Password: "ph", Status: 1, Locale: "zh-CN"}
	db.Create(user)
	db.Create(&model.UserRole{UserID: user.ID, RoleID: role.ID})
	sess := &model.AuthSession{ID: "s-jwt", UserID: user.ID, RefreshJTI: "jti", ExpiresAt: time.Now().Add(time.Hour)}
	db.Create(sess)
	if _, err := casbin.AddPolicy("user", "/api/protected", "GET"); err != nil {
		t.Fatal(err)
	}

	access, _ := token.GenerateAccessToken(user.ID, user.Username, []string{"user"}, "zh-CN", sess.ID)
	c, _ := newTestGin()
	c.Request = httptest.NewRequest(http.MethodGet, "/api/protected", nil)
	c.Request.Header.Set("Authorization", "Bearer "+access)
	JWTAuth(db)(c)
	if c.IsAborted() {
		t.Error("valid token should pass")
	}
	if c.GetUint64("current_user_id") != uint64(user.ID) {
		t.Error("user id not set in context")
	}
}

func TestAuditGETShortCircuit(t *testing.T) {
	c, _ := newTestGin()
	c.Request = httptest.NewRequest(http.MethodGet, "/api/x", nil)
	NewAuditMiddleware(nil)(c)
	if c.IsAborted() {
		t.Error("GET should not abort")
	}
}

func TestAuditWrite(t *testing.T) {
	db := newMWDB(t)
	repo := repository.NewOperationLogRepository(db)
	c, w := newTestGin()
	c.Request = httptest.NewRequest(http.MethodPost, "/api/users", nil)
	w.Code = 200
	w.Body.WriteString(`{"code":0,"msg":"ok"}`)
	NewAuditMiddleware(repo)(c)
	if c.IsAborted() {
		t.Error("audit should not abort")
	}
	time.Sleep(50 * time.Millisecond)
	var logs []model.OperationLog
	db.Find(&logs)
	found := false
	for _, l := range logs {
		if l.Path == "/api/users" && l.Status == 1 {
			found = true
		}
	}
	if !found {
		t.Error("audit log not written")
	}
}

func TestParseModuleAndVerb(t *testing.T) {
	cases := []struct {
		path, method, module, verb string
	}{
		{"/api/v1/users", "POST", "用户", "创建"},
		{"/api/v1/roles", "GET", "角色", "操作"},
		{"/api/v1/menus", "DELETE", "菜单", "删除"},
		{"/api/v1/permissions", "PUT", "权限", "更新"},
		{"/api/v1/auth/login", "POST", "认证", "登录"},
		{"/api/v1/user/profile", "GET", "个人中心", "操作"},
		{"/api/v1/unknown", "PATCH", "其他", "更新"},
	}
	for _, cs := range cases {
		m, v := parseModuleAndVerb(cs.path, cs.method)
		if m != cs.module || v != cs.verb {
			t.Errorf("parseModuleAndVerb(%q,%q) = %q,%q want %q,%q", cs.path, cs.method, m, v, cs.module, cs.verb)
		}
	}
}
