package model

import "testing"

func TestTableNames(t *testing.T) {
	cases := []struct {
		model  interface{ TableName() string }
		expect string
	}{
		{User{}, "sys_user"},
		{Role{}, "sys_role"},
		{Menu{}, "sys_menu"},
		{Permission{}, "sys_permission"},
		{CasbinRule{}, "casbin_rule"},
		{OperationLog{}, "sys_operation_log"},
		{UserRole{}, "user_roles"},
		{RoleMenu{}, "role_menus"},
		{UserAuth{}, "user_auths"},
		{AuthSession{}, "auth_sessions"},
	}
	for _, c := range cases {
		if got := c.model.TableName(); got != c.expect {
			t.Errorf("TableName() = %q, want %q", got, c.expect)
		}
	}
}

func TestMenuRemainingGetters(t *testing.T) {
	m := Menu{
		BaseModel: BaseModel{ID: 9},
		ParentID:  2,
		Name:      "n",
		Path:      "/p",
		Component: "c",
		Icon:      "i",
		Sort:      3,
		Type:      2,
		Perms:     "user:list",
		Hidden:    true, // 隐藏 -> GetVisible 应为 false
	}
	if m.GetComponent() != "c" {
		t.Errorf("GetComponent: %q", m.GetComponent())
	}
	if m.GetIcon() != "i" {
		t.Errorf("GetIcon: %q", m.GetIcon())
	}
	if m.GetPerms() != "user:list" {
		t.Errorf("GetPerms: %q", m.GetPerms())
	}
	if m.GetVisible() {
		t.Error("GetVisible 应为 false（Hidden=true）")
	}
	if m.GetType() != "C" {
		t.Errorf("GetType(2): %q", m.GetType())
	}

	// Hidden=false -> GetVisible 应为 true
	m2 := Menu{Hidden: false}
	if !m2.GetVisible() {
		t.Error("GetVisible 应为 true（Hidden=false）")
	}
}
