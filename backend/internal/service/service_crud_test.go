package service

import (
	"context"
	"testing"

	"sentinelgo/internal/model"
	"sentinelgo/internal/repository"
	"sentinelgo/pkg/utils"

	"gorm.io/gorm"
)

// setupAll 在隔离的 sqlite 内存库上构造各业务服务，便于覆盖 CRUD 编排逻辑。
func setupAll(t *testing.T) (*gorm.DB, *UserService, *RoleService, *MenuService, *PermissionService, *DashboardService, *OperationLogService) {
	t.Helper()
	db, _ := setupService(t)
	ur := repository.NewUserRepository(db)
	rr := repository.NewRoleRepository(db)
	mr := repository.NewMenuRepository(db)
	pr := repository.NewPermissionRepository(db)
	return db,
		NewUserService(ur, rr),
		NewRoleService(rr, mr),
		NewMenuService(mr, rr, db),
		NewPermissionService(pr, db),
		NewDashboardService(repository.NewDashboardRepository(db)),
		NewOperationLogService(repository.NewOperationLogRepository(db))
}

func TestUserServiceListAndGet(t *testing.T) {
	db, svc, _, _, _, _, _ := setupAll(t)
	u := mustCreateUser(t, db, "listuser", "ValidPassword123!", 1)

	list, total, err := svc.List(UserListReq{Keyword: "listuser"})
	if err != nil || total != 1 || len(list) != 1 {
		t.Fatalf("List 失败: total=%d len=%d err=%v", total, len(list), err)
	}
	if _, err := svc.GetByID(u.ID); err != nil {
		t.Errorf("GetByID 应成功: %v", err)
	}
	if _, err := svc.GetByID(99999); err == nil {
		t.Error("不存在的用户应报错")
	}
}

func TestUserServiceCreateAndErrors(t *testing.T) {
	_, svc, _, _, _, _, _ := setupAll(t)
	if err := svc.Create(&model.User{}); err == nil {
		t.Error("空用户名/密码应报错")
	}
	if err := svc.Create(&model.User{Username: "ok", Password: "ValidPassword123!"}); err != nil {
		t.Fatalf("Create 应成功: %v", err)
	}
	if err := svc.Create(&model.User{Username: "ok", Password: ""}); err == nil {
		t.Error("空密码应报错")
	}
}

func TestUserServiceUpdateDeleteStatus(t *testing.T) {
	db, svc, _, _, _, _, _ := setupAll(t)
	u := mustCreateUser(t, db, "upd", "ValidPassword123!", 1)

	if err := svc.Update(99999, &model.User{Nickname: "x"}); err == nil {
		t.Error("更新不存在用户应报错")
	}
	if err := svc.Update(u.ID, &model.User{Nickname: "新昵称", Email: "a@b.com"}); err != nil {
		t.Fatalf("Update 应成功: %v", err)
	}
	if err := svc.Update(u.ID, &model.User{Password: "NewPassword456!"}); err != nil {
		t.Fatalf("Update 密码应成功: %v", err)
	}
	var updated model.User
	if err := db.First(&updated, u.ID).Error; err != nil {
		t.Fatal(err)
	}
	if updated.Nickname != "新昵称" || updated.Email != "a@b.com" {
		t.Fatalf("更新未落库: %+v", updated)
	}
	if !utils.CheckPasswordHash("NewPassword456!", updated.Password) || utils.CheckPasswordHash("ValidPassword123!", updated.Password) {
		t.Fatal("新密码哈希未生效或旧密码仍有效")
	}
	if err := svc.UpdateStatus(u.ID, 2); err == nil {
		t.Error("非法状态应报错")
	}
	if err := svc.UpdateStatus(u.ID, 0); err != nil {
		t.Errorf("禁用应成功: %v", err)
	}
	if err := db.First(&updated, u.ID).Error; err != nil {
		t.Fatal(err)
	}
	if updated.Status != 0 {
		t.Fatal("禁用状态未落库")
	}
	if err := svc.Delete(u.ID); err != nil {
		t.Fatalf("Delete 应成功: %v", err)
	}
	var count int64
	if err := db.Model(&model.User{}).Where("id = ?", u.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("删除后用户仍存在")
	}
}

func TestUserServicePasswordAndProfile(t *testing.T) {
	db, svc, _, _, _, _, _ := setupAll(t)
	u := mustCreateUser(t, db, "prof", "ValidPassword123!", 1)
	if err := svc.UpdatePassword(u.ID, ""); err == nil {
		t.Error("空密码应报错")
	}
	if err := svc.UpdatePassword(u.ID, "AnotherPass789!"); err != nil {
		t.Fatalf("UpdatePassword 应成功: %v", err)
	}
	if err := svc.UpdateProfile(u.ID, "昵称", "e@e.com", "13800001111", "Asia/Shanghai", "en"); err != nil {
		t.Fatalf("UpdateProfile 应成功: %v", err)
	}
	if err := svc.UpdateAvatar(u.ID, "http://x/a.png"); err != nil {
		t.Fatalf("UpdateAvatar 应成功: %v", err)
	}
	var updated model.User
	if err := db.First(&updated, u.ID).Error; err != nil {
		t.Fatal(err)
	}
	if updated.Nickname != "昵称" || updated.Email != "e@e.com" || updated.Phone != "13800001111" || updated.Timezone != "Asia/Shanghai" || updated.Locale != "en" || updated.Avatar != "http://x/a.png" {
		t.Fatalf("个人资料字段未落库: %+v", updated)
	}
	if !utils.CheckPasswordHash("AnotherPass789!", updated.Password) || utils.CheckPasswordHash("ValidPassword123!", updated.Password) {
		t.Fatal("密码更新未生效")
	}
	if err := svc.UpdateProfile(99999, "x", "", "", "", ""); err == nil {
		t.Error("更新不存在用户资料应报错")
	}
}

func TestUserServiceAssignRolesAndGetRoles(t *testing.T) {
	db, svc, rsvc, _, _, _, _ := setupAll(t)
	u := mustCreateUser(t, db, "roleuser", "ValidPassword123!", 1)
	role := &model.Role{Name: "测试角色", Code: "trole", Desc: "d"}
	if err := rsvc.Create(role); err != nil {
		t.Fatalf("创建角色失败: %v", err)
	}
	if err := svc.AssignRoles(u.ID, []uint64{role.ID}); err != nil {
		t.Fatalf("AssignRoles 应成功: %v", err)
	}
	roles, err := svc.GetRoles(u.ID)
	if err != nil || len(roles) != 1 {
		t.Fatalf("GetRoles 应返回 1 个角色: %v len=%d", err, len(roles))
	}
	if roles[0].ID != role.ID {
		t.Fatalf("分配了错误角色: %+v", roles)
	}
	if err := svc.AssignRoles(u.ID, []uint64{}); err != nil {
		t.Fatal(err)
	}
	roles, err = svc.GetRoles(u.ID)
	if err != nil || len(roles) != 0 {
		t.Fatalf("清空角色未生效: %v %+v", err, roles)
	}
}

func TestRoleServiceCRUD(t *testing.T) {
	_, _, svc, _, _, _, _ := setupAll(t)
	if _, err := svc.List(""); err != nil {
		t.Errorf("List 应成功: %v", err)
	}
	role := &model.Role{Name: "测试角色", Code: "testrole", Desc: "d"}
	if err := svc.Create(role); err != nil {
		t.Fatalf("Create 应成功: %v", err)
	}
	if role.Status != 1 {
		t.Error("Create 应默认启用")
	}
	if _, err := svc.GetByID(99999); err == nil {
		t.Error("不存在角色应报错")
	}
	got, err := svc.GetByID(role.ID)
	if err != nil {
		t.Fatalf("GetByID 应成功: %v", err)
	}
	if err := svc.Update(role.ID, map[string]interface{}{"name": "新名", "desc": "新描述", "code": "newcode"}); err != nil {
		t.Fatalf("Update 应成功: %v", err)
	}
	updated, err := svc.GetByID(role.ID)
	if err != nil {
		t.Fatalf("更新后 GetByID 应成功: %v", err)
	}
	if updated.Name != "新名" || updated.Desc != "新描述" || updated.Code != "newcode" {
		t.Errorf("Update 字段未生效: %+v", updated)
	}
	_ = got
	if err := svc.UpdateStatus(role.ID, 2); err == nil {
		t.Error("非法状态应报错")
	}
	if err := svc.UpdateStatus(role.ID, 0); err != nil {
		t.Errorf("禁用应成功: %v", err)
	}
	updated, err = svc.GetByID(role.ID)
	if err != nil || updated.Status != 0 {
		t.Fatalf("禁用状态未生效: %v %+v", err, updated)
	}
	if err := svc.AssignPermissions(role.ID, []PermItem{{Path: "/api/v1/x", Method: "GET"}}); err != nil {
		t.Fatalf("AssignPermissions 应成功: %v", err)
	}
	if _, err := svc.GetMenus(role.ID); err != nil {
		t.Errorf("GetMenus 应成功: %v", err)
	}
	if err := svc.AssignMenus(role.ID, []uint64{}); err != nil {
		t.Errorf("AssignMenus 应成功: %v", err)
	}
	if err := svc.Delete(role.ID); err != nil {
		t.Fatalf("Delete 应成功: %v", err)
	}
	if _, err := svc.GetByID(role.ID); err == nil {
		t.Fatal("删除后角色仍存在")
	}
}

func TestMenuServiceCRUD(t *testing.T) {
	_, _, _, svc, _, _, _ := setupAll(t)
	if _, err := svc.ListTree(); err != nil {
		t.Errorf("ListTree 应成功: %v", err)
	}
	if _, err := svc.GetByID(99999); err == nil {
		t.Error("不存在菜单应报错")
	}
	menu := &model.Menu{Name: "菜单", Path: "/x", Component: "x", Icon: "i", Sort: 1, Type: 2, Status: 1}
	if err := svc.Create(menu); err != nil {
		t.Fatalf("Create 应成功: %v", err)
	}
	if err := svc.Update(menu.ID, &model.Menu{Name: "改名", Path: "/y", Component: "y", Icon: "j", Sort: 2, Type: 1, Hidden: true, Perms: "p"}); err != nil {
		t.Fatalf("Update 应成功: %v", err)
	}
	updated, err := svc.GetByID(menu.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "改名" || updated.Path != "/y" || updated.Component != "y" || updated.Icon != "j" || updated.Sort != 2 || updated.Type != 1 || !updated.Hidden || updated.Perms != "p" {
		t.Fatalf("菜单更新未落库: %+v", updated)
	}
	if err := svc.UpdateStatus(menu.ID, 0); err != nil {
		t.Errorf("UpdateStatus 应成功: %v", err)
	}
	updated, err = svc.GetByID(menu.ID)
	if err != nil || updated.Status != 0 {
		t.Fatalf("菜单禁用未生效: %v %+v", err, updated)
	}
	if _, err := svc.GetUserMenus(99999); err != nil {
		t.Errorf("GetUserMenus 应成功(兜底): %v", err)
	}
	if err := svc.InitDefaultMenus(); err != nil {
		t.Fatalf("InitDefaultMenus 应成功: %v", err)
	}
	if err := svc.InitDefaultMenus(); err != nil {
		t.Fatalf("InitDefaultMenus 幂等应成功: %v", err)
	}
	if err := svc.Delete(menu.ID); err != nil {
		t.Fatalf("Delete 应成功: %v", err)
	}
	if _, err := svc.GetByID(menu.ID); err == nil {
		t.Fatal("删除后菜单仍存在")
	}
}

func TestPermissionServiceCRUD(t *testing.T) {
	_, _, _, _, svc, _, _ := setupAll(t)
	if _, _, err := svc.List(PermissionListReq{}); err != nil {
		t.Errorf("List 应成功: %v", err)
	}
	if _, err := svc.GetByID(99999); err == nil {
		t.Error("不存在权限应报错")
	}
	perm := &model.Permission{Path: "/api/v1/p", Method: "GET", Description: "d"}
	if err := svc.Create(perm); err != nil {
		t.Fatalf("Create 应成功: %v", err)
	}
	if perm.Status != 1 {
		t.Error("Create 应默认启用")
	}
	if err := svc.Update(perm.ID, &model.Permission{Path: "/api/v1/p2", Method: "POST", Description: "d2"}); err != nil {
		t.Fatalf("Update 应成功: %v", err)
	}
	updated, err := svc.GetByID(perm.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Path != "/api/v1/p2" || updated.Method != "POST" || updated.Description != "d2" {
		t.Fatalf("权限更新未落库: %+v", updated)
	}
	if err := svc.Delete(perm.ID); err != nil {
		t.Fatalf("Delete 应成功: %v", err)
	}
	if _, err := svc.GetByID(perm.ID); err == nil {
		t.Fatal("删除后权限仍存在")
	}
	if err := svc.InitDefaultPermissions(); err != nil {
		t.Fatalf("InitDefaultPermissions 应成功: %v", err)
	}
	if err := svc.InitDefaultPermissions(); err != nil {
		t.Fatalf("InitDefaultPermissions 幂等应成功: %v", err)
	}
}

func TestDashboardServiceStats(t *testing.T) {
	_, _, _, _, _, svc, _ := setupAll(t)
	data, err := svc.Stats()
	if err != nil {
		t.Fatalf("Stats 应成功: %v", err)
	}
	if data == nil {
		t.Error("Stats 不应返回 nil")
	}
}

func TestOperationLogServiceList(t *testing.T) {
	_, _, _, _, _, _, svc := setupAll(t)
	if _, _, err := svc.List(context.Background(), repository.OpLogQuery{}); err != nil {
		t.Fatalf("List 应成功: %v", err)
	}
}
