import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, enableAutoUnmount } from '@vue/test-utils'
import { ElMessage, ElMessageBox, ElTree } from 'element-plus'
import { setLocale, i18n } from '../locales'
import { mountGlobals, testRouter } from '../test/mount'
import { mockApi, type ApiCall } from '../test/mockApi'
import * as auth from '../utils/auth'
import Login from './Login.vue'
import ChangePassword from './ChangePassword.vue'
import Dashboard from './Dashboard.vue'
import MonitorDashboard from './MonitorDashboard.vue'
import OperationLog from './OperationLog.vue'
import UserManagement from './UserManagement.vue'
import RoleManagement from './RoleManagement.vue'
import MenuManagement from './MenuManagement.vue'
import PermissionManagement from './PermissionManagement.vue'

enableAutoUnmount(afterEach)
let calls: ApiCall[]
let unexpected: string[]
function apiData(url: string, config: any): any {
  const route = `${(config.method || 'get').toUpperCase()} ${url}`
  switch (route) {
    case 'POST /api/login': return { code: 0, data: { access_token: 'a', refresh_token: 'r', username: 'u' } }
    case 'GET /users': return { code: 0, data: { list: [{ id: 7, username: 'alice', nickname: 'Alice', email: 'a@example.com', phone: '13800000000', status: 1, roles: [] }], total: 1 } }
    case 'GET /users/7/roles': return { code: 0, data: [{ id: 3, name: 'operator', code: 'operator' }] }
    case 'GET /roles': return { code: 0, data: [{ id: 3, name: 'operator', code: 'operator', status: 1 }] }
    case 'GET /roles/3/menus': return { code: 0, data: [{ id: 11 }] }
    case 'GET /menus/tree': return { code: 0, data: [{ id: 11, parent_id: 0, name: '首页', path: '/dashboard', sort: 1, type: 'C', status: 1, hidden: false }] }
    case 'GET /permissions': return { code: 0, data: { list: [{ id: 13, path: '/api/example', method: 'GET', description: 'example', status: 1 }], total: 1 } }
    case 'GET /dashboard/stats': return { code: 0, data: { total_users: 37, today_new_users: 0, total_roles: 1, total_permissions: 1, total_menus: 1, today_operations: 0, today_logins: 0, today_failed_logins: 0, today_failed_ops: 0, recent_operations: [], recent_failed_ops: [] } }
    case 'GET /monitor/stats': return { code: 0, data: { uptime_seconds: 100, num_goroutines: 10, memory_alloc_mb: 100, memory_total_mb: 200, num_cpu: 4, go_version: 'go1.23', http_metrics: [], http_total: 37, http_errors: 0, http_p99_seconds: 0.1 } }
    case 'GET /audit/logs': return { code: 0, data: { list: [{ id: 1, username: 'audit-user', module: '用户', operation: 'op', method: 'GET', path: '/api/example', ip: '1.1.1.1', status: 1, error_msg: '', user_agent: 'ua', trace_id: 'trace-example-42', created_at: '2026-01-01T00:00:00Z' }], total: 1 } }
    case 'POST /auth/password/initialize': case 'POST /auth/logout':
    case 'POST /users': case 'PUT /users/7': case 'DELETE /users/7':
    case 'PUT /users/7/password': case 'PUT /users/7/roles':
    case 'POST /roles': case 'DELETE /roles/3': case 'PUT /roles/3/menus':
    case 'POST /menus': case 'DELETE /menus/11': case 'DELETE /permissions/13': return { code: 0, data: {} }
    default:
      unexpected.push(route)
      throw new Error(`Unexpected request: ${route}`)
  }
}
beforeEach(async () => {
  localStorage.clear()
  setLocale('zh-CN')
  unexpected = []
  calls = mockApi(apiData)
  vi.spyOn(ElMessageBox, 'confirm').mockResolvedValue('confirm' as any)
  for (const method of ['success', 'error', 'warning'] as const) vi.spyOn(ElMessage, method).mockImplementation(() => undefined as any)
  await testRouter.replace('/login')
})
afterEach(() => {
  vi.restoreAllMocks()
  expect(unexpected).toEqual([])
})
function requests(method: string, url: string) { return calls.filter(c => c.method === method && c.url === url) }
function submitted(method: string, url: string, data: unknown) {
  expect(requests(method, url)).toEqual([expect.objectContaining({ data })])
}
async function click(wrapper: any, text: string) {
  const button = wrapper.findAll('button').find((b: any) => b.text().includes(text))
  if (!button) throw new Error(`按钮未找到: ${text}`)
  await button.trigger('click')
  await flushPromises()
}
async function input(wrapper: any, placeholder: string, value: string) {
  const dialog = wrapper.find('.el-dialog-stub')
  const scope = dialog.exists() ? dialog : wrapper
  const field = scope.findAll('input').find((i: any) => i.attributes('placeholder') === placeholder)
  if (!field) throw new Error(`输入框未找到: ${placeholder}`)
  await field.setValue(value)
}
async function fillPassword(w: any, password = 'NewPassword123456!', confirmation = password) {
  const fields = w.findAll('input[autocomplete]')
  expect(fields).toHaveLength(3)
  await fields[0].setValue('TemporaryPassword123!')
  await fields[1].setValue(password)
  await fields[2].setValue(confirmation)
}
describe('Login', () => {
  it('提交凭据、保存 token 并跳转首页', async () => {
    const w = mount(Login, mountGlobals)
    await input(w, '用户名', 'admin'); await input(w, '密码', 'password123')
    await click(w, '登录')
    submitted('POST', '/api/login', { username: 'admin', password: 'password123' })
    expect(auth.getToken()).toBe('a'); expect(auth.getRefreshToken()).toBe('r')
    expect(testRouter.currentRoute.value.path).toBe('/dashboard')
  })
  it('缺少凭据时提示且不提交', async () => {
    const w = mount(Login, mountGlobals)
    await click(w, '登录')
    expect(ElMessage.warning).toHaveBeenCalledWith(i18n.global.t('login.enterBoth'))
    expect(calls).toEqual([]); expect(auth.getToken()).toBeNull()
  })
})
describe('ChangePassword', () => {
  beforeEach(async () => {
    auth.saveAuth('old-access', 'old-refresh', 'admin'); auth.setPasswordChangeRequired(true)
    await testRouter.replace('/change-password')
  })
  it.each([
    ['密码不一致', 'NewPassword123456!', 'DifferentPassword123!', 'changePwd.mismatch'],
    ['密码太短', 'short', 'short', 'changePwd.pwdLenRule'],
  ])('%s：提示且不提交', async (_name, password, confirmation, message) => {
    const w = mount(ChangePassword, mountGlobals)
    await fillPassword(w, password, confirmation); await w.find('form').trigger('submit'); await flushPromises()
    expect(ElMessage.warning).toHaveBeenCalledWith(i18n.global.t(message))
    expect(calls).toEqual([]); expect(auth.getToken()).toBe('old-access')
  })
  it('保存成功提交三个密码字段、清除认证并重定向', async () => {
    const w = mount(ChangePassword, mountGlobals)
    await fillPassword(w); await w.find('form').trigger('submit'); await flushPromises()
    submitted('POST', '/auth/password/initialize', { current_password: 'TemporaryPassword123!', new_password: 'NewPassword123456!', confirm_password: 'NewPassword123456!' })
    expect(calls[0]?.authorization).toBe('Bearer old-access')
    expect(auth.getToken()).toBeNull(); expect(auth.getRefreshToken()).toBeNull(); expect(auth.mustChangePassword()).toBe(false)
    expect(testRouter.currentRoute.value.path).toBe('/login')
    expect(ElMessage.success).toHaveBeenCalledWith(i18n.global.t('changePwd.setSuccess'))
  })
  it('保存请求失败显示错误、保留认证和输入并恢复按钮', async () => {
    calls = mockApi(() => { throw new Error('network-down') })
    const w = mount(ChangePassword, mountGlobals)
    await fillPassword(w); await w.find('form').trigger('submit'); await flushPromises()
    expect(requests('POST', '/auth/password/initialize')).toHaveLength(1)
    expect(ElMessage.error).toHaveBeenCalledWith('network-down'); expect(ElMessage.success).not.toHaveBeenCalled()
    expect(auth.getToken()).toBe('old-access'); expect(testRouter.currentRoute.value.path).toBe('/change-password')
    expect((w.find('input[autocomplete="new-password"]').element as HTMLInputElement).value).toBe('NewPassword123456!')
    expect(w.find('.submit-button').classes()).not.toContain('is-loading')
  })
  it.each([false, true])('退出登录（请求失败=%s）仍清除认证并跳转', async fail => {
    if (fail) calls = mockApi(() => { throw new Error('network-down') })
    const w = mount(ChangePassword, mountGlobals)
    await w.find('.logout-button').trigger('click'); await flushPromises()
    expect(requests('POST', '/auth/logout')).toHaveLength(1)
    expect(auth.getToken()).toBeNull(); expect(auth.getRefreshToken()).toBeNull(); expect(auth.mustChangePassword()).toBe(false)
    expect(testRouter.currentRoute.value.path).toBe('/login')
  })
})
describe.each([
  ['Dashboard', Dashboard, '/dashboard/stats', '总用户数'],
  ['MonitorDashboard', MonitorDashboard, '/monitor/stats', 'Go 版本'],
] as const)('%s', (_name, component, url, label) => {
  it('显示接口数据且刷新重新请求', async () => {
    const w = mount(component, mountGlobals); await flushPromises()
    expect(w.text()).toContain(label); expect(w.text()).toContain('37')
    expect(requests('GET', url)).toHaveLength(1)
    await click(w, '刷新数据'); expect(requests('GET', url)).toHaveLength(2)
  })
})
describe('OperationLog', () => {
  it('加载日志并在详情中显示具体 trace', async () => {
    const w = mount(OperationLog, mountGlobals); await flushPromises()
    expect(w.text()).toContain('audit-user')
    await click(w, '详情'); expect(w.find('.el-drawer-stub').text()).toContain('trace-example-42')
  })
})
describe.each([
  ['UserManagement', UserManagement, '/users', 'keyword'], ['RoleManagement', RoleManagement, '/roles', 'keyword'],
  ['PermissionManagement', PermissionManagement, '/permissions', 'keyword'], ['OperationLog', OperationLog, '/audit/logs', 'username'],
] as const)('%s 查询', (_name, component, url, field) => {
  it('搜索传入筛选值，重置清空并重新查询', async () => {
    const w = mount(component, mountGlobals); await flushPromises()
    await w.find('.search-input input').setValue('needle'); await click(w, '搜索')
    expect(requests('GET', url).at(-1)?.params).toMatchObject({ [field]: 'needle' })
    await click(w, '重置'); expect(requests('GET', url)).toHaveLength(3)
    expect(requests('GET', url).at(-1)?.params).toMatchObject({ [field]: '' })
    expect((w.find('.search-input input').element as HTMLInputElement).value).toBe('')
  })
})
describe('UserManagement', () => {
  it('新增提交输入值，成功关闭弹窗并刷新列表', async () => {
    const w = mount(UserManagement, mountGlobals); await flushPromises(); await click(w, '新增')
    await input(w, '请输入用户名', 'newuser'); await input(w, '请输入密码', 'ValidPassword123!')
    await click(w.find('.el-dialog-stub'), '确定')
    submitted('POST', '/users', expect.objectContaining({ username: 'newuser', password: 'ValidPassword123!' }))
    expect(w.find('.el-dialog-stub').exists()).toBe(false); expect(requests('GET', '/users')).toHaveLength(2)
  })
  it('编辑昵称提交原用户 ID，空密码不发送', async () => {
    const w = mount(UserManagement, mountGlobals); await flushPromises(); await click(w, '编辑')
    await input(w, '昵称', 'Edited Alice'); await click(w.find('.el-dialog-stub'), '确定')
    submitted('PUT', '/users/7', expect.objectContaining({ nickname: 'Edited Alice', username: 'alice' }))
    expect(requests('PUT', '/users/7')[0]?.data).not.toHaveProperty('password')
    expect(requests('GET', '/users')).toHaveLength(2); expect(w.find('.el-dialog-stub').exists()).toBe(false)
  })
  it.each([true, false])('分配角色提交实际勾选的角色 ID（选中=%s）', async selected => {
    const w = mount(UserManagement, mountGlobals); await flushPromises(); await click(w, '分配角色')
    expect(requests('GET', '/users/7/roles')).toHaveLength(1)
    const checkbox = w.find('.el-dialog-stub input[type="checkbox"]')
    expect((checkbox.element as HTMLInputElement).checked).toBe(true)
    await checkbox.setValue(selected); await click(w.find('.el-dialog-stub'), '确定')
    submitted('PUT', '/users/7/roles', { role_ids: selected ? [3] : [] }); expect(w.find('.el-dialog-stub').exists()).toBe(false)
  })
  it('修改密码提交目标用户及新密码', async () => {
    const w = mount(UserManagement, mountGlobals); await flushPromises(); await click(w, '修改密码')
    await input(w, '请输入新密码', 'ValidPassword123!'); await click(w.find('.el-dialog-stub'), '确定')
    submitted('PUT', '/users/7/password', { password: 'ValidPassword123!' }); expect(w.find('.el-dialog-stub').exists()).toBe(false)
  })
})
describe('RoleManagement', () => {
  it('新增角色提交名称和编码并刷新', async () => {
    const w = mount(RoleManagement, mountGlobals); await flushPromises(); await click(w, '新增')
    await input(w, '角色名称', 'reviewer'); await input(w, '角色编码', 'reviewer'); await click(w.find('.el-dialog-stub'), '确定')
    submitted('POST', '/roles', { name: 'reviewer', code: 'reviewer', desc: '' })
    expect(requests('GET', '/roles')).toHaveLength(2); expect(w.find('.el-dialog-stub').exists()).toBe(false)
  })
  it.each([true, false])('分配菜单预选已有菜单并提交用户选择（选中=%s）', async selected => {
    const w = mount(RoleManagement, mountGlobals); await flushPromises(); await click(w, '分配菜单')
    expect(requests('GET', '/roles/3/menus')).toHaveLength(1)
    const tree = w.findComponent(ElTree)
    expect(tree.vm.getCheckedKeys()).toEqual([11])
    await tree.find('input[type="checkbox"]').setValue(selected)
    await click(w.find('.el-dialog-stub'), '确定'); submitted('PUT', '/roles/3/menus', { menu_ids: selected ? [11] : [] })
    expect(w.find('.el-dialog-stub').exists()).toBe(false)
  })
  it('未选择权限时提示且不发送请求', async () => {
    const w = mount(RoleManagement, mountGlobals); await flushPromises(); await click(w, '分配权限')
    await click(w.find('.el-dialog-stub'), '确定')
    expect(ElMessage.warning).toHaveBeenCalledWith(i18n.global.t('role.permAtLeastOne'))
    expect(requests('PUT', '/roles/3/permissions')).toEqual([]); expect(w.find('.el-dialog-stub').exists()).toBe(true)
  })
})
describe('MenuManagement', () => {
  it('加载菜单数据，刷新和重置重新请求菜单树', async () => {
    const w = mount(MenuManagement, mountGlobals); await flushPromises(); expect(w.text()).toContain('首页')
    await click(w, '搜索'); await click(w, '重置'); expect(requests('GET', '/menus/tree')).toHaveLength(3)
  })
  it('新增目录提交名称、路径及数值类型', async () => {
    const w = mount(MenuManagement, mountGlobals); await flushPromises(); await click(w, '新增根菜单')
    await input(w, '请输入菜单名称', '新菜单'); await input(w, '如 /system/users', '/sys/x'); await click(w.find('.el-dialog-stub'), '确定')
    submitted('POST', '/menus', expect.objectContaining({ name: '新菜单', path: '/sys/x', type: 1, parent_id: 0 }))
    expect(requests('GET', '/menus/tree')).toHaveLength(2); expect(w.find('.el-dialog-stub').exists()).toBe(false)
  })
})
describe('PermissionManagement', () => {
  it('点击新增打开空表单', async () => {
    const w = mount(PermissionManagement, mountGlobals); await flushPromises()
    expect(w.find('.el-dialog-stub').exists()).toBe(false); await click(w, '新增')
    expect(w.find('.el-dialog-stub input').exists()).toBe(true)
    expect((w.find('.el-dialog-stub input').element as HTMLInputElement).value).toBe('')
    expect(requests('POST', '/permissions')).toEqual([])
  })
})
describe.each([
  ['用户', UserManagement, '/users', '/users/7'], ['角色', RoleManagement, '/roles', '/roles/3'],
  ['菜单', MenuManagement, '/menus/tree', '/menus/11'], ['权限', PermissionManagement, '/permissions', '/permissions/13'],
] as const)('%s 删除', (_name, component, listUrl, deleteUrl) => {
  it('确认后删除正确 ID 并刷新列表', async () => {
    const w = mount(component, mountGlobals); await flushPromises(); await click(w, '删除')
    expect(ElMessageBox.confirm).toHaveBeenCalledTimes(1); expect(requests('DELETE', deleteUrl)).toHaveLength(1)
    expect(requests('GET', listUrl)).toHaveLength(2)
    expect(ElMessage.success).toHaveBeenCalledWith(i18n.global.t('user.deleteSuccess'))
  })
  it('取消删除不发送请求且不刷新', async () => {
    vi.mocked(ElMessageBox.confirm).mockRejectedValueOnce('cancel')
    const w = mount(component, mountGlobals); await flushPromises(); await click(w, '删除')
    expect(ElMessageBox.confirm).toHaveBeenCalledTimes(1); expect(requests('DELETE', deleteUrl)).toEqual([])
    expect(requests('GET', listUrl)).toHaveLength(1); expect(ElMessage.success).not.toHaveBeenCalled()
  })
})
