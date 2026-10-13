/**
 * 角色 API 模块 — 角色 CRUD、菜单分配、权限策略分配。
 *
 * Role 对应后端 Role 模型，包含：
 *   - code: 角色编码（用于 Casbin 策略中的 subject）
 *   - 菜单分配：通过 role_menus 关联表
 *   - 权限分配：直接操作 Casbin p 规则
 */
import request from '@/utils/request'

/** 角色接口定义 */
export interface Role {
  id: number
  name: string   // 角色名称（如 "管理员"）
  code: string   // 角色编码（如 "admin"，用于 Casbin subject）
  desc: string   // 角色描述
  status: number // 1=启用, 0=禁用
  created_at: string
  updated_at: string
}

/** 查询角色列表（支持关键词搜索） */
export function getRoles(params?: { keyword?: string }) {
  return request.get('/roles', { params })
}

/** 按 ID 查询角色 */
export function getRole(id: number) {
  return request.get(`/roles/${id}`)
}

/** 创建角色 */
export function createRole(data: Partial<Role>) {
  return request.post('/roles', data)
}

/** 更新角色 */
export function updateRole(id: number, data: Partial<Role>) {
  return request.put(`/roles/${id}`, data)
}

/** 级联删除角色（含关联数据和 Casbin 策略） */
export function deleteRole(id: number) {
  return request.delete(`/roles/${id}`)
}

/** 更新角色启用/禁用状态 */
export function updateRoleStatus(id: number, status: number) {
  return request.put(`/roles/${id}/status`, { status })
}

/** 获取角色拥有的菜单列表 */
export function getRoleMenus(id: number) {
  return request.get(`/roles/${id}/menus`)
}

/** 为角色分配菜单（替换式，清除旧关联再写入） */
export function assignRoleMenus(id: number, menu_ids: number[]) {
  return request.put(`/roles/${id}/menus`, { menu_ids })
}

/** 为角色分配 Casbin 权限策略（先清除后添加） */
export function assignRolePermissions(id: number, permissions: { path: string; method: string }[]) {
  return request.put(`/roles/${id}/permissions`, { permissions })
}
