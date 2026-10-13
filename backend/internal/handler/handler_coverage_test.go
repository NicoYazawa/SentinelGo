package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"gorm.io/gorm"

	"sentinelgo/internal/model"
	"sentinelgo/internal/repository"
	"sentinelgo/internal/service"
	"sentinelgo/pkg/utils"
)

// setupAllHandlers 在 setupHandler 的集成环境上构建其余业务 Handler，并返回底层 db 供造数。
func setupAllHandlers(t *testing.T) (*gorm.DB, *service.UserService, *service.RoleService, *service.MenuService, *service.PermissionService, *UserHandler, *RoleHandler, *MenuHandler, *PermissionHandler, *DashboardHandler, *OperationLogHandler) {
	t.Helper()
	db, _, _ := setupHandler(t)
	ur := repository.NewUserRepository(db)
	rr := repository.NewRoleRepository(db)
	mr := repository.NewMenuRepository(db)
	pr := repository.NewPermissionRepository(db)
	userSvc := service.NewUserService(ur, rr)
	roleSvc := service.NewRoleService(rr, mr)
	menuSvc := service.NewMenuService(mr, rr, db)
	permSvc := service.NewPermissionService(pr, db)
	dashSvc := service.NewDashboardService(repository.NewDashboardRepository(db))
	opSvc := service.NewOperationLogService(repository.NewOperationLogRepository(db))
	return db, userSvc, roleSvc, menuSvc, permSvc,
		NewUserHandler(userSvc),
		NewRoleHandler(roleSvc),
		NewMenuHandler(menuSvc),
		NewPermissionHandler(permSvc),
		NewDashboardHandler(dashSvc),
		NewOperationLogHandler(opSvc)
}

func hctxRaw(t *testing.T, method, path string, body []byte, userID uint64, sessionID string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	if userID != 0 {
		c.Set("current_user_id", userID)
	}
	if sessionID != "" {
		c.Set("current_session_id", sessionID)
	}
	return c, w
}

func TestUserHandlerCRUD(t *testing.T) {
	db, _, _, _, _, h, _, _, _, _, _ := setupAllHandlers(t)
	_ = mustCreateUser(t, db, "handleruser", "ValidPassword123!", 1)

	// List（带关键词）
	c, w := hctx(t, http.MethodGet, "/api/v1/users?keyword=handleruser", nil, 0, "")
	h.List(c)
	if r := decodeH(t, w); r.Code != 0 || r.Data == nil {
		t.Fatalf("User.List 失败: %+v", decodeH(t, w))
	}
	// Get 存在 / 不存在
	gc, gw := hctx(t, http.MethodGet, "/api/v1/users/1", nil, 0, "")
	h.Get(gc)
	if decodeH(t, gw).Code != 0 {
		t.Error("User.Get 应成功")
	}
	nc, nw := hctx(t, http.MethodGet, "/api/v1/users/99999", nil, 0, "")
	h.Get(nc)
	if decodeH(t, nw).Code == 0 {
		t.Error("User.Get 不存在应失败")
	}
	// Create 成功 / 缺密码失败
	cc, cw := hctx(t, http.MethodPost, "/api/v1/users", map[string]string{"username": "newu", "password": "ValidPassword123!", "nickname": "n"}, 0, "")
	h.Create(cc)
	if decodeH(t, cw).Code != 0 {
		t.Errorf("User.Create 失败: %+v", decodeH(t, cw))
	}
	bc, bw := hctxRaw(t, http.MethodPost, "/api/v1/users", []byte(`{"username":"x"}`), 0, "")
	h.Create(bc)
	if decodeH(t, bw).Code == 0 {
		t.Error("User.Create 缺密码应失败")
	}
	// Update 存在 / 不存在
	uc, uw := hctx(t, http.MethodPut, "/api/v1/users/1", map[string]interface{}{"nickname": "upd"}, 0, "")
	h.Update(uc)
	if decodeH(t, uw).Code != 0 {
		t.Error("User.Update 应成功")
	}
	uc2, uw2 := hctx(t, http.MethodPut, "/api/v1/users/99999", map[string]interface{}{"nickname": "x"}, 0, "")
	h.Update(uc2)
	if decodeH(t, uw2).Code == 0 {
		t.Error("User.Update 不存在应失败")
	}
	// UpdateStatus 成功 / 缺 status 失败
	sc, sw := hctx(t, http.MethodPut, "/api/v1/users/1/status", map[string]interface{}{"status": 0}, 0, "")
	h.UpdateStatus(sc)
	if decodeH(t, sw).Code != 0 {
		t.Error("User.UpdateStatus 应成功")
	}
	sc2, sw2 := hctxRaw(t, http.MethodPut, "/api/v1/users/1/status", []byte(`{}`), 0, "")
	h.UpdateStatus(sc2)
	if decodeH(t, sw2).Code == 0 {
		t.Error("User.UpdateStatus 缺 status 应失败")
	}
	// UpdatePassword 成功 / 缺密码失败
	pc, pw := hctx(t, http.MethodPut, "/api/v1/users/1/password", map[string]string{"password": "AnotherPass789!"}, 0, "")
	h.UpdatePassword(pc)
	if decodeH(t, pw).Code != 0 {
		t.Error("User.UpdatePassword 应成功")
	}
	pc2, pw2 := hctxRaw(t, http.MethodPut, "/api/v1/users/1/password", []byte(`{}`), 0, "")
	h.UpdatePassword(pc2)
	if decodeH(t, pw2).Code == 0 {
		t.Error("User.UpdatePassword 缺密码应失败")
	}
	// AssignRoles / GetRoles
	ac, aw := hctx(t, http.MethodPut, "/api/v1/users/1/roles", map[string]interface{}{"role_ids": []uint64{1}}, 0, "")
	h.AssignRoles(ac)
	if decodeH(t, aw).Code != 0 {
		t.Error("User.AssignRoles 应成功")
	}
	rc, rw := hctx(t, http.MethodGet, "/api/v1/users/1/roles", nil, 0, "")
	h.GetRoles(rc)
	if decodeH(t, rw).Code != 0 {
		t.Error("User.GetRoles 应成功")
	}
	// Delete
	dc, dw := hctx(t, http.MethodDelete, "/api/v1/users/1", nil, 0, "")
	h.Delete(dc)
	if decodeH(t, dw).Code != 0 {
		t.Error("User.Delete 应成功")
	}
}

func TestUserHandlerMeAndProfile(t *testing.T) {
	db, _, _, _, _, h, _, _, _, _, _ := setupAllHandlers(t)
	me := mustCreateUser(t, db, "meuser", "ValidPassword123!", 1)
	mustAssignRole(t, db, me.ID, "admin")

	mc, mw := hctx(t, http.MethodGet, "/api/v1/users/me", nil, me.ID, "")
	h.GetCurrentUser(mc)
	if decodeH(t, mw).Code != 0 {
		t.Errorf("GetCurrentUser 失败: %+v", decodeH(t, mw))
	}
	pc, pw := hctx(t, http.MethodGet, "/api/v1/profile", nil, me.ID, "")
	h.GetProfile(pc)
	if decodeH(t, pw).Code != 0 {
		t.Error("GetProfile 应成功")
	}
	upc, upw := hctx(t, http.MethodPut, "/api/v1/profile", map[string]string{"nickname": "新昵称", "email": "a@b.com", "locale": "en-US"}, me.ID, "")
	h.UpdateProfile(upc)
	if decodeH(t, upw).Code != 0 {
		t.Errorf("UpdateProfile 失败: %+v", decodeH(t, upw))
	}
	upc2, upw2 := hctxRaw(t, http.MethodPut, "/api/v1/profile", []byte(`{bad`), me.ID, "")
	h.UpdateProfile(upc2)
	if decodeH(t, upw2).Code == 0 {
		t.Error("UpdateProfile 坏 JSON 应失败")
	}
}

func TestUserHandlerUploadAvatarError(t *testing.T) {
	_, _, _, _, _, h, _, _, _, _, _ := setupAllHandlers(t)
	c, w := hctxRaw(t, http.MethodPost, "/api/v1/profile/avatar", []byte(`{}`), 1, "")
	h.UploadAvatar(c)
	if decodeH(t, w).Code == 0 {
		t.Error("UploadAvatar 无文件应失败")
	}
}

func TestRoleHandlerCRUD(t *testing.T) {
	db, _, _, _, _, _, h, _, _, _, _ := setupAllHandlers(t)
	lc, lw := hctx(t, http.MethodGet, "/api/v1/roles", nil, 0, "")
	h.List(lc)
	if decodeH(t, lw).Code != 0 {
		t.Error("Role.List 应成功")
	}
	cc, cw := hctx(t, http.MethodPost, "/api/v1/roles", map[string]interface{}{"name": "角色A", "code": "rolea", "desc": "d"}, 0, "")
	h.Create(cc)
	if decodeH(t, cw).Code != 0 {
		t.Errorf("Role.Create 失败: %+v", decodeH(t, cw))
	}
	var created model.Role
	if err := db.Where("code = ?", "rolea").First(&created).Error; err != nil {
		t.Fatal(err)
	}
	if created.Name != "角色A" || created.Desc != "d" {
		t.Fatalf("创建字段未落库: %+v", created)
	}
	path := fmt.Sprintf("/api/v1/roles/%d", created.ID)
	gc, gw := hctx(t, http.MethodGet, path, nil, 0, "")
	h.Get(gc)
	if decodeH(t, gw).Code != 0 {
		t.Error("Role.Get 应成功")
	}
	var result model.Role
	if err := json.Unmarshal(decodeH(t, gw).Data, &result); err != nil {
		t.Fatal(err)
	}
	if result.ID != created.ID || result.Code != "rolea" {
		t.Fatalf("Get 返回错误角色: %+v", result)
	}
	nc, nw := hctx(t, http.MethodGet, "/api/v1/roles/99999", nil, 0, "")
	h.Get(nc)
	if decodeH(t, nw).Code == 0 {
		t.Error("Role.Get 不存在应失败")
	}
	uc, uw := hctx(t, http.MethodPut, path, map[string]interface{}{"name": "新名"}, 0, "")
	h.Update(uc)
	if decodeH(t, uw).Code != 0 {
		t.Error("Role.Update 应成功")
	}
	if err := db.First(&result, created.ID).Error; err != nil {
		t.Fatal(err)
	}
	if result.Name != "新名" {
		t.Fatal("角色更新未落库")
	}
	dc, dw := hctx(t, http.MethodDelete, path, nil, 0, "")
	h.Delete(dc)
	if decodeH(t, dw).Code != 0 {
		t.Error("Role.Delete 应成功")
	}
	var count int64
	if err := db.Model(&model.Role{}).Where("id = ?", created.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("删除后角色仍存在")
	}
	if err := db.Model(&model.Role{}).Where("code = ?", "admin").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("误删了测试预置的 admin 角色")
	}
	// role id=2 为 setupHandler 内置的 user 角色，仍存在
	sc, sw := hctx(t, http.MethodPut, "/api/v1/roles/2/status", map[string]interface{}{"status": 0}, 0, "")
	h.UpdateStatus(sc)
	if decodeH(t, sw).Code != 0 {
		t.Error("Role.UpdateStatus 应成功")
	}
	var userRole model.Role
	if err := db.First(&userRole, 2).Error; err != nil {
		t.Fatal(err)
	}
	if userRole.Status != 0 {
		t.Fatal("角色禁用未落库")
	}
	menu := model.Menu{Name: "assigned", Path: "/assigned", Type: 2, Status: 1}
	if err := db.Create(&menu).Error; err != nil {
		t.Fatal(err)
	}
	ac, aw := hctx(t, http.MethodPut, "/api/v1/roles/2/menus", map[string]interface{}{"menu_ids": []uint64{menu.ID}}, 0, "")
	h.AssignMenus(ac)
	if decodeH(t, aw).Code != 0 {
		t.Error("Role.AssignMenus 应成功")
	}
	gmc, gmw := hctx(t, http.MethodGet, "/api/v1/roles/2/menus", nil, 0, "")
	h.GetMenus(gmc)
	if decodeH(t, gmw).Code != 0 {
		t.Error("Role.GetMenus 应成功")
	}
	var menus []model.Menu
	if err := json.Unmarshal(decodeH(t, gmw).Data, &menus); err != nil {
		t.Fatal(err)
	}
	if len(menus) != 1 || menus[0].ID != menu.ID {
		t.Fatalf("菜单分配未生效: %+v", menus)
	}
	pc, pw := hctx(t, http.MethodPut, "/api/v1/roles/2/permissions", map[string]interface{}{"permissions": []map[string]string{{"path": "/api/x", "method": "GET"}}}, 0, "")
	h.AssignPermissions(pc)
	if decodeH(t, pw).Code != 0 {
		t.Errorf("Role.AssignPermissions 失败: %+v", decodeH(t, pw))
	}
	pc2, pw2 := hctxRaw(t, http.MethodPut, "/api/v1/roles/2/permissions", []byte(`{bad`), 0, "")
	h.AssignPermissions(pc2)
	if decodeH(t, pw2).Code == 0 {
		t.Error("Role.AssignPermissions 坏 JSON 应失败")
	}
}

func TestMenuHandlerCRUD(t *testing.T) {
	db, _, _, _, _, _, _, h, _, _, _ := setupAllHandlers(t)
	lc, lw := hctx(t, http.MethodGet, "/api/v1/menus", nil, 0, "")
	h.List(lc)
	if decodeH(t, lw).Code != 0 {
		t.Error("Menu.List 应成功")
	}
	cc, cw := hctx(t, http.MethodPost, "/api/v1/menus", map[string]interface{}{"name": "菜单A", "path": "/a", "component": "a", "sort": 1, "type": 2, "status": 1}, 0, "")
	h.Create(cc)
	if decodeH(t, cw).Code != 0 {
		t.Errorf("Menu.Create 失败: %+v", decodeH(t, cw))
	}
	gc, gw := hctx(t, http.MethodGet, "/api/v1/menus/1", nil, 0, "")
	h.Get(gc)
	if decodeH(t, gw).Code != 0 {
		t.Error("Menu.Get 应成功")
	}
	nc, nw := hctx(t, http.MethodGet, "/api/v1/menus/99999", nil, 0, "")
	h.Get(nc)
	if decodeH(t, nw).Code == 0 {
		t.Error("Menu.Get 不存在应失败")
	}
	uc, uw := hctx(t, http.MethodPut, "/api/v1/menus/1", map[string]interface{}{"name": "改名"}, 0, "")
	h.Update(uc)
	if decodeH(t, uw).Code != 0 {
		t.Error("Menu.Update 应成功")
	}
	sc, sw := hctx(t, http.MethodPut, "/api/v1/menus/1/status", map[string]interface{}{"status": 0}, 0, "")
	h.UpdateStatus(sc)
	if decodeH(t, sw).Code != 0 {
		t.Error("Menu.UpdateStatus 应成功")
	}
	dc, dw := hctx(t, http.MethodDelete, "/api/v1/menus/1", nil, 0, "")
	h.Delete(dc)
	if decodeH(t, dw).Code != 0 {
		t.Error("Menu.Delete 应成功")
	}
	// GetUserMenus：构造一个已分配角色的用户
	u := mustCreateUser(t, db, "menuuser", "ValidPassword123!", 1)
	mustAssignRole(t, db, u.ID, "user")
	allowed := model.Menu{BaseModel: model.BaseModel{ID: 21}, Name: "allowed", Path: "/allowed", Type: 2, Status: 1}
	other := model.Menu{BaseModel: model.BaseModel{ID: 22}, Name: "other", Path: "/other", Type: 2, Status: 1}
	for _, menu := range []*model.Menu{&allowed, &other} {
		if err := db.Create(menu).Error; err != nil {
			t.Fatal(err)
		}
	}
	var role model.Role
	if err := db.Where("code = ?", "user").First(&role).Error; err != nil {
		t.Fatal(err)
	}
	if err := repository.NewRoleRepository(db).AssignRoleMenus(role.ID, []uint64{allowed.ID}); err != nil {
		t.Fatal(err)
	}
	mc, mw := hctx(t, http.MethodGet, "/api/v1/menus/user", nil, u.ID, "")
	h.GetUserMenus(mc)
	if decodeH(t, mw).Code != 0 {
		t.Errorf("Menu.GetUserMenus 失败: %+v", decodeH(t, mw))
	}
	var tree []utils.MenuTreeResp
	if err := json.Unmarshal(decodeH(t, mw).Data, &tree); err != nil {
		t.Fatal(err)
	}
	if len(tree) != 1 || tree[0].ID != allowed.ID || tree[0].Path != allowed.Path {
		t.Fatalf("应只返回用户角色已分配的菜单，实际: %+v", tree)
	}
}

func TestPermissionHandlerCRUD(t *testing.T) {
	_, _, _, _, _, _, _, _, h, _, _ := setupAllHandlers(t)
	lc, lw := hctx(t, http.MethodGet, "/api/v1/permissions", nil, 0, "")
	h.List(lc)
	if decodeH(t, lw).Code != 0 {
		t.Error("Permission.List 应成功")
	}
	cc, cw := hctx(t, http.MethodPost, "/api/v1/permissions", map[string]interface{}{"path": "/api/p", "method": "GET", "description": "d"}, 0, "")
	h.Create(cc)
	if decodeH(t, cw).Code != 0 {
		t.Errorf("Permission.Create 失败: %+v", decodeH(t, cw))
	}
	gc, gw := hctx(t, http.MethodGet, "/api/v1/permissions/1", nil, 0, "")
	h.Get(gc)
	if decodeH(t, gw).Code != 0 {
		t.Error("Permission.Get 应成功")
	}
	nc, nw := hctx(t, http.MethodGet, "/api/v1/permissions/99999", nil, 0, "")
	h.Get(nc)
	if decodeH(t, nw).Code == 0 {
		t.Error("Permission.Get 不存在应失败")
	}
	uc, uw := hctx(t, http.MethodPut, "/api/v1/permissions/1", map[string]interface{}{"description": "new"}, 0, "")
	h.Update(uc)
	if decodeH(t, uw).Code != 0 {
		t.Error("Permission.Update 应成功")
	}
	dc, dw := hctx(t, http.MethodDelete, "/api/v1/permissions/1", nil, 0, "")
	h.Delete(dc)
	if decodeH(t, dw).Code != 0 {
		t.Error("Permission.Delete 应成功")
	}
}

func TestDashboardAndAudit(t *testing.T) {
	_, _, _, _, _, _, _, _, _, dh, oh := setupAllHandlers(t)
	dc, dw := hctx(t, http.MethodGet, "/api/v1/dashboard/stats", nil, 0, "")
	dh.Stats(dc)
	if decodeH(t, dw).Code != 0 {
		t.Errorf("Dashboard.Stats 失败: %+v", decodeH(t, dw))
	}
	ac, aw := hctx(t, http.MethodGet, "/api/v1/audit/logs?page=1&page_size=10&status=0", nil, 0, "")
	oh.List(ac)
	if decodeH(t, aw).Code != 0 {
		t.Errorf("OperationLog.List 失败: %+v", decodeH(t, aw))
	}
}

func TestUserHandlerUploadAvatarSuccess(t *testing.T) {
	db, _, _, _, _, h, _, _, _, _, _ := setupAllHandlers(t)
	me := mustCreateUser(t, db, "avuser", "ValidPassword123!", 1)
	old := avatarBaseDir
	avatarBaseDir = t.TempDir()
	defer func() { avatarBaseDir = old }()

	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	fw, err := mw.CreateFormFile("avatar", "a.png")
	if err != nil {
		t.Fatal(err)
	}
	var imageData bytes.Buffer
	if err := png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(imageData.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/profile/avatar", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	c.Set("current_user_id", me.ID)
	h.UploadAvatar(c)
	if r := decodeH(t, rec); r.Code != 0 {
		t.Errorf("UploadAvatar 应成功: %+v", r)
	}
	var result struct {
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.Unmarshal(decodeH(t, rec).Data, &result); err != nil {
		t.Fatal(err)
	}
	if result.AvatarURL == "" {
		t.Fatal("上传未返回头像 URL")
	}
	saved, err := os.ReadFile(filepath.Join(avatarBaseDir, "avatars", filepath.Base(result.AvatarURL)))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(saved, imageData.Bytes()) {
		t.Fatal("保存的头像内容与上传 PNG 不一致")
	}
	var updated model.User
	if err := db.First(&updated, me.ID).Error; err != nil {
		t.Fatal(err)
	}
	if updated.Avatar != result.AvatarURL {
		t.Fatalf("头像 URL 未落库: %q != %q", updated.Avatar, result.AvatarURL)
	}
}

func TestBadJSONBinding(t *testing.T) {
	_, _, _, _, _, uh, rh, mh, ph, _, _ := setupAllHandlers(t)

	// User.Update 坏 JSON
	uc, uw := hctxRaw(t, http.MethodPut, "/api/v1/users/1", []byte(`{bad`), 0, "")
	uh.Update(uc)
	if decodeH(t, uw).Code == 0 {
		t.Error("User.Update 坏 JSON 应失败")
	}
	// Role.Create / Update 坏 JSON
	rc, rw := hctxRaw(t, http.MethodPost, "/api/v1/roles", []byte(`{bad`), 0, "")
	rh.Create(rc)
	if decodeH(t, rw).Code == 0 {
		t.Error("Role.Create 坏 JSON 应失败")
	}
	ruc, ruw := hctxRaw(t, http.MethodPut, "/api/v1/roles/1", []byte(`{bad`), 0, "")
	rh.Update(ruc)
	if decodeH(t, ruw).Code == 0 {
		t.Error("Role.Update 坏 JSON 应失败")
	}
	// Menu.Create / Update 坏 JSON
	mc, mw := hctxRaw(t, http.MethodPost, "/api/v1/menus", []byte(`{bad`), 0, "")
	mh.Create(mc)
	if decodeH(t, mw).Code == 0 {
		t.Error("Menu.Create 坏 JSON 应失败")
	}
	muc, muw := hctxRaw(t, http.MethodPut, "/api/v1/menus/1", []byte(`{bad`), 0, "")
	mh.Update(muc)
	if decodeH(t, muw).Code == 0 {
		t.Error("Menu.Update 坏 JSON 应失败")
	}
	// Permission.Create / Update 坏 JSON
	pc, pw := hctxRaw(t, http.MethodPost, "/api/v1/permissions", []byte(`{bad`), 0, "")
	ph.Create(pc)
	if decodeH(t, pw).Code == 0 {
		t.Error("Permission.Create 坏 JSON 应失败")
	}
	puc, puw := hctxRaw(t, http.MethodPut, "/api/v1/permissions/1", []byte(`{bad`), 0, "")
	ph.Update(puc)
	if decodeH(t, puw).Code == 0 {
		t.Error("Permission.Update 坏 JSON 应失败")
	}
}

func TestHandlerErrorPaths(t *testing.T) {
	db, _, _, _, _, h, rh, _, _, _, _ := setupAllHandlers(t)
	_ = db

	// GetCurrentUser / GetProfile 用户不存在
	gmc, gmw := hctx(t, http.MethodGet, "/api/v1/users/me", nil, 99999, "")
	h.GetCurrentUser(gmc)
	if decodeH(t, gmw).Code == 0 {
		t.Error("GetCurrentUser 不存在用户应失败")
	}
	gpc, gpw := hctx(t, http.MethodGet, "/api/v1/profile", nil, 99999, "")
	h.GetProfile(gpc)
	if decodeH(t, gpw).Code == 0 {
		t.Error("GetProfile 不存在用户应失败")
	}
	// Role.AssignPermissions 角色不存在
	apc, apw := hctx(t, http.MethodPut, "/api/v1/roles/99999/permissions", map[string]interface{}{"permissions": []map[string]string{{"path": "/x", "method": "GET"}}}, 0, "")
	rh.AssignPermissions(apc)
	if decodeH(t, apw).Code == 0 {
		t.Error("Role.AssignPermissions 角色不存在应失败")
	}
}

func TestMonitorStats(t *testing.T) {
	// 注入自定义 Registry，覆盖计数器（含 5xx）/ 直方图（P99）分支
	reg := prometheus.NewRegistry()
	cnt := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "http_requests_total"}, []string{"path", "method", "status"})
	cnt.WithLabelValues("/", "GET", "200").Add(3)
	cnt.WithLabelValues("/", "GET", "500").Add(1)
	if err := reg.Register(cnt); err != nil {
		t.Fatal(err)
	}
	hist := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Buckets: []float64{0.1, 0.5, 1},
	}, []string{"path", "method", "status"})
	hist.WithLabelValues("/", "GET", "200").Observe(0.2)
	if err := reg.Register(hist); err != nil {
		t.Fatal(err)
	}

	old := monitorGatherer
	monitorGatherer = reg
	defer func() { monitorGatherer = old }()

	h := NewMonitorHandler()
	c, w := hctx(t, http.MethodGet, "/api/v1/monitor/stats", nil, 0, "")
	h.Stats(c)
	if decodeH(t, w).Code != 0 {
		t.Errorf("Monitor.Stats 失败: %+v", decodeH(t, w))
	}
	var stats MonitorStats
	if err := json.Unmarshal(decodeH(t, w).Data, &stats); err != nil {
		t.Fatal(err)
	}
	if stats.HTTPTotal != 4 || stats.HTTPErrors != 1 || stats.HTTPP99Seconds != 0.5 {
		t.Fatalf("监控聚合错误: total=%v errors=%v p99=%v", stats.HTTPTotal, stats.HTTPErrors, stats.HTTPP99Seconds)
	}
	counts := map[string]float64{}
	for _, metric := range stats.HTTPMetrics {
		if metric.Path != "/" || metric.Method != "GET" {
			t.Fatalf("指标标签错误: %+v", metric)
		}
		counts[metric.Status] = metric.Count
	}
	if len(stats.HTTPMetrics) != 2 || counts["200"] != 3 || counts["500"] != 1 {
		t.Fatalf("HTTP 指标错误: %+v", stats.HTTPMetrics)
	}
}
