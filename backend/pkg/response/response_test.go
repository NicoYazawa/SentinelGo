package response

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"sentinelgo/pkg/i18n"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestGetErrorMsg(t *testing.T) {
	if GetErrorMsg(CodeSuccess, i18n.EnUS) != "success" {
		t.Error("success msg wrong")
	}
	if !contains(GetErrorMsg(ErrUnauthorized, i18n.ZhCN), "登录") {
		t.Error("zh unauthorized msg wrong")
	}
	// unknown code -> fallback
	if GetErrorMsg(99999, i18n.EnUS) != "未知业务错误" {
		t.Error("unknown code fallback wrong")
	}
	// unknown lang -> default (zh)
	if GetErrorMsg(ErrForbidden, i18n.Lang("fr")) != "权限不足，拒绝访问" {
		t.Error("unknown lang fallback wrong")
	}
}

func contains(s, sub string) bool {
	return bytes.Contains([]byte(s), []byte(sub))
}

func newTestCtx(body string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/x", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	return c, w
}

func TestSuccess(t *testing.T) {
	c, w := newTestCtx("")
	Success(c, gin.H{"k": "v"})
	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Code != CodeSuccess || resp.Msg != "success" {
		t.Errorf("unexpected success response: %+v", resp)
	}
}

func TestFail(t *testing.T) {
	c, w := newTestCtx("")
	i18n.SetInContext(c, i18n.EnUS)
	Fail(c, ErrForbidden, "")
	var resp Response
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != ErrForbidden {
		t.Errorf("code wrong: %d", resp.Code)
	}
	if resp.Msg != "Access denied: insufficient permissions" {
		t.Errorf("msg wrong: %q", resp.Msg)
	}
	// custom msg overrides mapping (fresh context/recorder)
	c3, w3 := newTestCtx("")
	i18n.SetInContext(c3, i18n.EnUS)
	Fail(c3, ErrForbidden, "custom")
	var resp3 Response
	_ = json.Unmarshal(w3.Body.Bytes(), &resp3)
	if resp3.Msg != "custom" {
		t.Errorf("custom msg wrong: %q", resp3.Msg)
	}
}

type bindTarget struct {
	Name string `json:"name" validate:"required"`
}

func TestShouldBindJSON(t *testing.T) {
	// valid
	c, _ := newTestCtx(`{"name":"ok"}`)
	var t1 bindTarget
	if !ShouldBindJSON(c, &t1) {
		t.Error("expected valid bind")
	}
	// invalid JSON -> false
	c2, _ := newTestCtx(`not-json`)
	var t2 bindTarget
	if ShouldBindJSON(c2, &t2) {
		t.Error("expected invalid bind to fail")
	}
}
