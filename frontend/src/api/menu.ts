/**
 * 菜单 API 模块 — 菜单树查询、CRUD、用户菜单。
 *
 * MenuItem 接口对应后端 MenuTreeResp 结构：
 *   - type: 'M'=目录, 'C'=菜单, 'F'=按钮
 *   - hidden: 是否在侧边栏隐藏（与后端 Visible 字段逻辑取反）
 *   - children: 子菜单（树形结构）
 */
import request from '@/utils/request'

/** 菜单项接口定义 */
export interface MenuItem {
  id: number
  parent_id: number
  name: string
  path: string
  component?: string   // 前端组件路径（仅 type=C 时有效）
  icon?: string        // Element Plus 图标名
  sort: number
  type: string         // M=目录, C=菜单, F=按钮
  perms?: string       // 权限标识（仅 type=F 时有效）
  status: number       // 1=启用, 0=禁用
  hidden: boolean      // 是否在侧边栏隐藏
  children?: MenuItem[]
  created_at?: string
  updated_at?: string
}

/** 获取菜单列表（扁平结构） */
export function getMenus() {
  return request.get('/menus')
}

/** 获取菜单树（嵌套结构） */
export function getMenuTree() {
  return request.get('/menus/tree')
}

/** 按 ID 查询单个菜单 */
export function getMenu(id: number) {
  return request.get(`/menus/${id}`)
}

/** 创建菜单 */
export function createMenu(data: Partial<MenuItem>) {
  return request.post('/menus', data)
}

/** 更新菜单（部分更新） */
export function updateMenu(id: number, data: Partial<MenuItem>) {
  return request.put(`/menus/${id}`, data)
}

/** 删除菜单 */
export function deleteMenu(id: number) {
  return request.delete(`/menus/${id}`)
}

/** 更新菜单启用/禁用状态 */
export function updateMenuStatus(id: number, status: number) {
  return request.put(`/menus/${id}/status`, { status })
}

/** 获取当前用户可访问的菜单树（按角色过滤） */
export function getUserMenus() {
  return request.get('/user/menus')
}
