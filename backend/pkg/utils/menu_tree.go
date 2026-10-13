package utils

import "sort"

// MenuTreeResp 是返回给前端的菜单树节点结构。
// 使用嵌套的 Children 字段表示父子关系，前端可直接渲染为 el-tree。
type MenuTreeResp struct {
	ID        uint64          `json:"id"`                  // 菜单 ID
	ParentID  uint64          `json:"parent_id"`           // 父菜单 ID（0 为顶级）
	Name      string          `json:"name"`                // 菜单名称
	Path      string          `json:"path"`                // 前端路由路径
	Component string          `json:"component,omitempty"` // 前端组件路径
	Icon      string          `json:"icon,omitempty"`      // 菜单图标
	Sort      int             `json:"sort"`                // 排序号（越小越靠前）
	Type      string          `json:"type"`                // 类型：M=目录 C=菜单 F=按钮
	Perms     string          `json:"perms,omitempty"`     // 权限标识
	Visible   bool            `json:"visible"`             // 是否在侧边栏显示
	Children  []*MenuTreeResp `json:"children,omitempty"`  // 子菜单列表
}

// MenuItem 是从各层数据源获取菜单信息的统一接口。
// Model 层的菜单实体需实现此接口，方可传入 BuildMenuTree。
type MenuItem interface {
	GetID() uint64
	GetParentID() uint64
	GetName() string
	GetPath() string
	GetComponent() string
	GetIcon() string
	GetSort() int
	GetType() string
	GetPerms() string
	GetVisible() bool
}

// MenuInput 是 MenuItem 接口的简单实现，用于测试或临时数据。
type MenuInput struct {
	ID        uint64
	ParentID  uint64
	Name      string
	Path      string
	Component string
	Icon      string
	Sort      int
	Type      string
	Perms     string
	Visible   bool
}

func (m MenuInput) GetID() uint64        { return m.ID }
func (m MenuInput) GetParentID() uint64  { return m.ParentID }
func (m MenuInput) GetName() string      { return m.Name }
func (m MenuInput) GetPath() string      { return m.Path }
func (m MenuInput) GetComponent() string { return m.Component }
func (m MenuInput) GetIcon() string      { return m.Icon }
func (m MenuInput) GetSort() int         { return m.Sort }
func (m MenuInput) GetType() string      { return m.Type }
func (m MenuInput) GetPerms() string     { return m.Perms }
func (m MenuInput) GetVisible() bool     { return m.Visible }

// BuildMenuTree 将扁平的菜单列表构建为树形结构。
// 使用泛型[T MenuItem]支持任意实现了 MenuItem 接口的类型。
//
// 算法：O(n) 两遍扫描
//  1. 第一遍：将每个菜单转为树节点存入 HashMap
//  2. 第二遍：根据 ParentID 将子节点挂到父节点的 Children 列表
//  3. 按 Sort 字段对每层节点排序
//
// ParentID=0 的节点为顶级节点，返回的 tree 即为顶层菜单列表。
func BuildMenuTree[T MenuItem](menus []T) []*MenuTreeResp {
	// 第一遍：构建 HashMap（ID → 节点）
	menuMap := make(map[uint64]*MenuTreeResp, len(menus))
	var tree []*MenuTreeResp

	for _, m := range menus {
		node := &MenuTreeResp{
			ID:        m.GetID(),
			ParentID:  m.GetParentID(),
			Name:      m.GetName(),
			Path:      m.GetPath(),
			Component: m.GetComponent(),
			Icon:      m.GetIcon(),
			Sort:      m.GetSort(),
			Type:      m.GetType(),
			Perms:     m.GetPerms(),
			Visible:   m.GetVisible(),
			Children:  make([]*MenuTreeResp, 0),
		}
		menuMap[m.GetID()] = node
	}

	// 第二遍：建立父子关系
	for _, m := range menus {
		node := menuMap[m.GetID()]
		if m.GetParentID() == 0 {
			// 顶级节点直接加入结果
			tree = append(tree, node)
		} else {
			// 子节点挂到父节点下
			if parent, exists := menuMap[m.GetParentID()]; exists {
				parent.Children = append(parent.Children, node)
			}
		}
	}

	// 按 Sort 排序（顶级节点 + 每层的子节点）
	sort.Slice(tree, func(i, j int) bool { return tree[i].Sort < tree[j].Sort })
	for _, node := range menuMap {
		sort.Slice(node.Children, func(i, j int) bool {
			return node.Children[i].Sort < node.Children[j].Sort
		})
	}

	return tree
}
