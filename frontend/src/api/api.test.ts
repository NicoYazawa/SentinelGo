import { describe, it, expect, vi, beforeEach } from 'vitest'

// 所有 api 模块都从 '@/utils/request' 导入默认实例；用一个可控 mock 覆盖之。
const mockRequest = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  delete: vi.fn(),
}))
vi.mock('@/utils/request', () => ({ default: mockRequest }))

import * as userApi from './user'
import * as roleApi from './role'
import * as menuApi from './menu'
import * as permissionApi from './permission'
import * as auditApi from './audit'
import * as dashboardApi from './dashboard'
import * as monitorApi from './monitor'

beforeEach(() => {
  mockRequest.get.mockReset()
  mockRequest.post.mockReset()
  mockRequest.put.mockReset()
  mockRequest.delete.mockReset()
})

describe('user api', () => {
  it('getUsers 带 params', () => {
    userApi.getUsers({ keyword: 'a', page: 2, page_size: 10 })
    expect(mockRequest.get).toHaveBeenCalledWith('/users', { params: { keyword: 'a', page: 2, page_size: 10 } })
  })
  it('getUser / createUser / updateUser / deleteUser', () => {
    userApi.getUser(5); expect(mockRequest.get).toHaveBeenCalledWith('/users/5')
    userApi.createUser({ username: 'x' }); expect(mockRequest.post).toHaveBeenCalledWith('/users', { username: 'x' })
    userApi.updateUser(5, { nickname: 'n' }); expect(mockRequest.put).toHaveBeenCalledWith('/users/5', { nickname: 'n' })
    userApi.deleteUser(5); expect(mockRequest.delete).toHaveBeenCalledWith('/users/5')
  })
  it('updateUserStatus / updateUserPassword / getUserRoles / assignUserRoles', () => {
    userApi.updateUserStatus(5, 0); expect(mockRequest.put).toHaveBeenCalledWith('/users/5/status', { status: 0 })
    userApi.updateUserPassword(5, 'p'); expect(mockRequest.put).toHaveBeenCalledWith('/users/5/password', { password: 'p' })
    userApi.getUserRoles(5); expect(mockRequest.get).toHaveBeenCalledWith('/users/5/roles')
    userApi.assignUserRoles(5, [1, 2]); expect(mockRequest.put).toHaveBeenCalledWith('/users/5/roles', { role_ids: [1, 2] })
  })
  it('getProfile / updateProfile', () => {
    userApi.getProfile(); expect(mockRequest.get).toHaveBeenCalledWith('/user/profile')
    userApi.updateProfile({ nickname: 'n' }); expect(mockRequest.put).toHaveBeenCalledWith('/user/profile', { nickname: 'n' })
  })
})

describe('role api', () => {
  it('CRUD + status + menus + permissions', () => {
    roleApi.getRoles({ keyword: 'a' }); expect(mockRequest.get).toHaveBeenCalledWith('/roles', { params: { keyword: 'a' } })
    roleApi.getRole(3); expect(mockRequest.get).toHaveBeenCalledWith('/roles/3')
    roleApi.createRole({ name: 'r' }); expect(mockRequest.post).toHaveBeenCalledWith('/roles', { name: 'r' })
    roleApi.updateRole(3, { name: 'r' }); expect(mockRequest.put).toHaveBeenCalledWith('/roles/3', { name: 'r' })
    roleApi.deleteRole(3); expect(mockRequest.delete).toHaveBeenCalledWith('/roles/3')
    roleApi.updateRoleStatus(3, 1); expect(mockRequest.put).toHaveBeenCalledWith('/roles/3/status', { status: 1 })
    roleApi.getRoleMenus(3); expect(mockRequest.get).toHaveBeenCalledWith('/roles/3/menus')
    roleApi.assignRoleMenus(3, [1]); expect(mockRequest.put).toHaveBeenCalledWith('/roles/3/menus', { menu_ids: [1] })
    roleApi.assignRolePermissions(3, [{ path: '/x', method: 'GET' }]); expect(mockRequest.put).toHaveBeenCalledWith('/roles/3/permissions', { permissions: [{ path: '/x', method: 'GET' }] })
  })
})

describe('menu api', () => {
  it('CRUD + tree + user menus', () => {
    menuApi.getMenus(); expect(mockRequest.get).toHaveBeenCalledWith('/menus')
    menuApi.getMenuTree(); expect(mockRequest.get).toHaveBeenCalledWith('/menus/tree')
    menuApi.getMenu(2); expect(mockRequest.get).toHaveBeenCalledWith('/menus/2')
    menuApi.createMenu({ name: 'm' }); expect(mockRequest.post).toHaveBeenCalledWith('/menus', { name: 'm' })
    menuApi.updateMenu(2, { name: 'm' }); expect(mockRequest.put).toHaveBeenCalledWith('/menus/2', { name: 'm' })
    menuApi.deleteMenu(2); expect(mockRequest.delete).toHaveBeenCalledWith('/menus/2')
    menuApi.updateMenuStatus(2, 0); expect(mockRequest.put).toHaveBeenCalledWith('/menus/2/status', { status: 0 })
    menuApi.getUserMenus(); expect(mockRequest.get).toHaveBeenCalledWith('/user/menus')
  })
})

describe('permission api', () => {
  it('CRUD', () => {
    permissionApi.getPermissions({ page: 1, method: 'GET' }); expect(mockRequest.get).toHaveBeenCalledWith('/permissions', { params: { page: 1, method: 'GET' } })
    permissionApi.getPermission(1); expect(mockRequest.get).toHaveBeenCalledWith('/permissions/1')
    permissionApi.createPermission({ path: '/x' }); expect(mockRequest.post).toHaveBeenCalledWith('/permissions', { path: '/x' })
    permissionApi.updatePermission(1, { path: '/x' }); expect(mockRequest.put).toHaveBeenCalledWith('/permissions/1', { path: '/x' })
    permissionApi.deletePermission(1); expect(mockRequest.delete).toHaveBeenCalledWith('/permissions/1')
  })
})

describe('audit / dashboard / monitor api', () => {
  it('getOperationLogs', () => {
    auditApi.getOperationLogs({ username: 'a' }); expect(mockRequest.get).toHaveBeenCalledWith('/audit/logs', { params: { username: 'a' } })
  })
  it('getDashboardStats', () => {
    dashboardApi.getDashboardStats(); expect(mockRequest.get).toHaveBeenCalledWith('/dashboard/stats')
  })
  it('getMonitorStats', () => {
    monitorApi.getMonitorStats(); expect(mockRequest.get).toHaveBeenCalledWith('/monitor/stats')
  })
})
