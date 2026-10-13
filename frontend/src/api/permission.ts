/**
 * 权限 API 模块 — 权限定义的 CRUD。
 *
 * Permission 对应后端 Permission 表（权限定义），与 Casbin 策略是不同维度：
 *   - 权限定义：前端展示用，描述有哪些 API 端点可被授权
 *   - Casbin 策略：运行时鉴权用，p 规则定义角色-路径-方法的映射
 */
import request from '@/utils/request'

/** 权限定义接口 */
export interface Permission {
  id: number
  path: string       // API 路径，如 /api/v1/users
  method: string     // HTTP 方法，如 GET/POST/PUT/DELETE
  description: string // 权限描述
  status: number     // 1=启用, 0=禁用
  created_at: string
  updated_at: string
}

/** 分页查询权限列表（支持关键词和方法过滤） */
export function getPermissions(params: { page?: number; page_size?: number; keyword?: string; method?: string }) {
  return request.get('/permissions', { params })
}

/** 按 ID 查询权限 */
export function getPermission(id: number) {
  return request.get(`/permissions/${id}`)
}

/** 创建权限定义 */
export function createPermission(data: Partial<Permission>) {
  return request.post('/permissions', data)
}

/** 更新权限定义 */
export function updatePermission(id: number, data: Partial<Permission>) {
  return request.put(`/permissions/${id}`, data)
}

/** 删除权限定义 */
export function deletePermission(id: number) {
  return request.delete(`/permissions/${id}`)
}
