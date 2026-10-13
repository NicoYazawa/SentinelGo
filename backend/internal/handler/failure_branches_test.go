package handler

import (
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"sentinelgo/internal/model"
	"sentinelgo/pkg/response"
)

func TestUnavailableStorage(t *testing.T) {
	db, _, _, _, _, u, r, m, p, d, o := setupAllHandlers(t)
	sqlDB, e := db.DB()
	if e != nil {
		t.Fatal(e)
	}
	if e = sqlDB.Close(); e != nil {
		t.Fatal(e)
	}
	cases := []struct {
		name, body string
		fn         gin.HandlerFunc
		code       int
		msg        string
	}{
		{"users list", "", u.List, response.ErrSystemError, "closed"},
		{"users create", `{"username":"new","password":"ValidPassword123!"}`, u.Create, response.ErrSystemError, "closed"},
		{"users delete", "", u.Delete, response.ErrSystemError, "closed"},
		{"users status", `{"status":0}`, u.UpdateStatus, response.ErrSystemError, "closed"},
		{"users password", `{"password":"ValidPassword123!"}`, u.UpdatePassword, response.ErrSystemError, "closed"},
		{"users assign", `{"role_ids":[1]}`, u.AssignRoles, response.ErrSystemError, "closed"},
		{"users roles", "", u.GetRoles, response.ErrSystemError, "closed"},
		{"profile", `{"nickname":"changed"}`, u.UpdateProfile, response.ErrSystemError, "closed"},
		{"roles list", "", r.List, response.ErrSystemError, "closed"},
		{"roles create", `{"code":"new","name":"new"}`, r.Create, response.ErrSystemError, "closed"},
		{"roles update", `{"name":"changed"}`, r.Update, response.ErrSystemError, "closed"},
		{"roles delete", "", r.Delete, response.ErrSystemError, "closed"},
		{"roles status", `{"status":0}`, r.UpdateStatus, response.ErrSystemError, "closed"},
		{"roles assign", `{"menu_ids":[1]}`, r.AssignMenus, response.ErrSystemError, "closed"},
		{"roles menus", "", r.GetMenus, response.ErrSystemError, "closed"},
		{"menus list", "", m.List, response.ErrSystemError, "closed"},
		{"menus create", `{"name":"new","path":"/new"}`, m.Create, response.ErrSystemError, "closed"},
		{"menus update", `{"name":"changed"}`, m.Update, response.ErrSystemError, "closed"},
		{"menus delete", "", m.Delete, response.ErrSystemError, "closed"},
		{"menus status", `{"status":0}`, m.UpdateStatus, response.ErrSystemError, "closed"},
		{"user menus", "", m.GetUserMenus, response.ErrMenuNotAvailable, "获取用户菜单失败"},
		{"permissions list", "", p.List, response.ErrSystemError, "closed"},
		{"permissions create", `{"path":"/new","method":"GET"}`, p.Create, response.ErrSystemError, "closed"},
		{"permissions update", `{"description":"changed"}`, p.Update, response.ErrSystemError, "closed"},
		{"permissions delete", "", p.Delete, response.ErrSystemError, "closed"},
		{"dashboard", "", d.Stats, response.ErrSystemError, "closed"},
		{"audit", "", o.List, response.ErrSystemError, "closed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, w := hctxRaw(t, "POST", "/1", []byte(tc.body), 1, "")
			setIDParam(c, "/1")
			tc.fn(c)
			got := decodeH(t, w)
			if got.Code != tc.code || !strings.Contains(got.Msg, tc.msg) {
				t.Fatalf("got %+v; want %d and %q", got, tc.code, tc.msg)
			}
			if string(got.Data) != "{}" {
				t.Fatalf("failure leaked data: %s", got.Data)
			}
		})
	}
}
func TestAssignmentBindingFailures(t *testing.T) {
	db, _, _, _, _, u, r, _, _, _, _ := setupAllHandlers(t)
	cases := []struct {
		name, msg string
		fn        gin.HandlerFunc
	}{{"user roles", "请提供角色列表", u.AssignRoles}, {"role menus", "参数错误", r.AssignMenus}, {"role status", "请提供状态", r.UpdateStatus}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, w := hctxRaw(t, "PUT", "/1", []byte(`{"status":"invalid","role_ids":"invalid","menu_ids":"invalid"}`), 0, "")
			setIDParam(c, "/1")
			tc.fn(c)
			got := decodeH(t, w)
			if got.Code != response.ErrParamInvalid || got.Msg != tc.msg {
				t.Fatalf("unexpected rejection %+v", got)
			}
		})
	}
	var count int64
	if e := db.Model(&model.UserRole{}).Count(&count).Error; e != nil {
		t.Fatal(e)
	}
	if count != 0 {
		t.Fatal("invalid assignment wrote roles")
	}
}
