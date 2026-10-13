package model

import (
	"sentinelgo/pkg/utils"
)

// Menu 实现 utils.MenuItem 接口，使得 GORM 实体可以直接传入 BuildMenuTree 构建树形结构。
//
// GetType 方法通过 utils.MenuTypeString 将数值类型转为字母编码：
//
//	1 (目录) → "M"
//	2 (菜单) → "C"
//	3 (按钮) → "F"
//
// GetVisible 取反 Hidden 字段：隐藏的菜单在前端不显示，但权限检查仍然生效。
func (m Menu) GetID() uint64        { return m.ID }
func (m Menu) GetParentID() uint64  { return m.ParentID }
func (m Menu) GetName() string      { return m.Name }
func (m Menu) GetPath() string      { return m.Path }
func (m Menu) GetComponent() string { return m.Component }
func (m Menu) GetIcon() string      { return m.Icon }
func (m Menu) GetSort() int         { return m.Sort }
func (m Menu) GetType() string      { return utils.MenuTypeString(m.Type) }
func (m Menu) GetPerms() string     { return m.Perms }
func (m Menu) GetVisible() bool     { return !m.Hidden } // Hidden=true 的菜单前端不显示
