/**
 * 用户 API 模块 — 用户 CRUD、角色分配、密码修改。
 *
 * User 接口对应后端 User 模型：
 *   - roles: 用户拥有的角色列表（多对多关联）
 *   - is_admin: 是否为管理员（角色 code 包含 "admin"）
 */
import request from '@/utils/request'

/** 用户接口定义 */
export interface User {
  id: number
  username: string
  nickname: string
  email: string
  phone: string
  avatar: string     // 头像 URL（相对路径）
  status: number     // 1=启用, 0=禁用
  is_admin: boolean  // 是否管理员
  timezone?: string  // 个人时区（IANA，如 Asia/Shanghai）
  created_at: string
  updated_at: string
  roles?: RoleItem[]
}

/** 角色简要信息（用于用户角色展示） */
export interface RoleItem {
  id: number
  name: string
  code: string
}

/** 分页查询用户列表（支持关键词搜索） */
export function getUsers(params: { keyword?: string; page?: number; page_size?: number }) {
  return request.get('/users', { params })
}

/** 按 ID 查询用户 */
export function getUser(id: number) {
  return request.get(`/users/${id}`)
}

/** 创建用户 */
export function createUser(data: Partial<User>) {
  return request.post('/users', data)
}

/** 更新用户 */
export function updateUser(id: number, data: Partial<User>) {
  return request.put(`/users/${id}`, data)
}

/** 删除用户 */
export function deleteUser(id: number) {
  return request.delete(`/users/${id}`)
}

/** 更新用户启用/禁用状态 */
export function updateUserStatus(id: number, status: number) {
  return request.put(`/users/${id}/status`, { status })
}

/** 修改用户密码（管理员操作） */
export function updateUserPassword(id: number, password: string) {
  return request.put(`/users/${id}/password`, { password })
}

/** 获取用户拥有的角色列表 */
export function getUserRoles(id: number) {
  return request.get(`/users/${id}/roles`)
}

/** 为用户分配角色（替换式，会同步 Casbin 策略） */
export function assignUserRoles(id: number, role_ids: number[]) {
  return request.put(`/users/${id}/roles`, { role_ids })
}

/** 当前登录用户的个人信息（含时区），用于「个人中心」 */
export interface Profile {
  id: number
  username: string
  nickname: string
  email: string
  phone: string
  avatar: string
  timezone: string
  locale: string
  status: number
  is_admin: boolean
}

/** 获取当前登录用户的个人信息 */
export function getProfile() {
  return request.get<unknown, { code: number; data: Profile; msg: string }>('/user/profile')
}

/** 更新当前登录用户的个人信息（昵称/邮箱/手机号/时区/语言），按需传参 */
export function updateProfile(data: Partial<Pick<Profile, 'nickname' | 'email' | 'phone' | 'timezone' | 'locale'>>) {
  return request.put<unknown, { code: number; data: null; msg: string }>('/user/profile', data)
}
