import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { ElDatePicker, ElMessage, ElSelect, ElSwitch, ElTreeSelect } from 'element-plus'
import { mountGlobals, testRouter } from '../test/mount'
import { mockApi, type ApiCall } from '../test/mockApi'
import { i18n, setLocale } from '../locales'
import * as auth from '../utils/auth'
import Login from './Login.vue'
import PermissionManagement from './PermissionManagement.vue'
import RoleManagement from './RoleManagement.vue'
import UserManagement from './UserManagement.vue'
import MenuManagement from './MenuManagement.vue'
import OperationLog from './OperationLog.vue'
import Dashboard from './Dashboard.vue'
import MonitorDashboard from './MonitorDashboard.vue'
import ProfileDialog from '../components/ProfileDialog.vue'
import Sidebar from '../components/Sidebar.vue'

enableAutoUnmount(afterEach)
let calls: ApiCall[]
let unexpected: string[]
const role = { id: 3, name: 'operator', code: 'operator', desc: 'original', status: 1 }
const menu = { id: 11, parent_id: 0, name: '首页', path: '/dashboard', component: 'Dashboard', icon: 'Home', sort: 1, type: 'C', status: 1, hidden: false }
function install(handlers: Record<string, any>) {
  calls = mockApi((url, config) => {
    const key = `${config.method.toUpperCase()} ${url}`
    if (!(key in handlers)) { unexpected.push(key); throw new Error(`Unexpected request: ${key}`) }
    const data = typeof handlers[key] === 'function' ? handlers[key]() : handlers[key]
    return { code: 0, data }
  })
}
function sent(method: string, url: string) { return calls.filter(c => c.method === method && c.url === url) }
async function click(w: any, text: string) {
  const button = w.findAll('button').find((b: any) => b.text().includes(text))
  if (!button) throw new Error(`Missing button: ${text}`)
  await button.trigger('click'); await flushPromises()
}
async function field(w: any, placeholder: string, value: string) {
  const input = w.findAll('input').find((el: any) => el.attributes('placeholder') === placeholder)
  if (!input) throw new Error(`Missing input: ${placeholder}`)
  await input.setValue(value)
}
beforeEach(async () => {
  localStorage.clear(); unexpected = []; setLocale('zh-CN')
  for (const method of ['success', 'error', 'warning'] as const) vi.spyOn(ElMessage, method).mockImplementation(() => undefined as any)
  await testRouter.replace('/login')
})
afterEach(() => { vi.restoreAllMocks(); localStorage.clear(); expect(unexpected).toEqual([]) })

describe('login edge cases', () => {
  async function submit(w: any) {
    await field(w, '用户名', 'admin'); await field(w, '密码', 'Password123!')
    await w.find('form').trigger('submit'); await flushPromises()
    expect(sent('POST', '/api/login')[0]?.data).toEqual({ username: 'admin', password: 'Password123!' })
    expect(w.find('.login-button').classes()).not.toContain('is-loading')
  }
  it('initial password users are redirected before normal login success', async () => {
    install({ 'POST /api/login': { access_token: 'a', refresh_token: 'r', username: 'admin', password_change_required: true } })
    await testRouter.replace('/login?redirect=/system/users')
    await submit(mount(Login, mountGlobals))
    expect(auth.mustChangePassword()).toBe(true)
    expect(auth.getToken()).toBe('a'); expect(testRouter.currentRoute.value.path).toBe('/change-password')
    expect(ElMessage.success).not.toHaveBeenCalled()
  })
  it('successful login honors the requested destination', async () => {
    install({ 'POST /api/login': { access_token: 'a', refresh_token: 'r', username: 'admin' } })
    await testRouter.replace('/login?redirect=/system/users')
    await submit(mount(Login, mountGlobals))
    expect(testRouter.currentRoute.value.path).toBe('/system/users')
    expect(auth.mustChangePassword()).toBe(false)
  })
  it.each(['invalid credentials', undefined])('business rejection keeps credentials and reports %s', async msg => {
    calls = mockApi(() => ({ code: 50000, msg }))
    const w = mount(Login, mountGlobals); await submit(w)
    expect(ElMessage.error).toHaveBeenCalledWith(msg || i18n.global.t('login.failed'))
    expect(auth.getToken()).toBeNull(); expect(testRouter.currentRoute.value.path).toBe('/login')
    expect((w.find('input[type=password]').element as HTMLInputElement).value).toBe('Password123!')
  })
  it('transport rejection restores the button and reports network failure', async () => {
    install({ 'POST /api/login': () => { throw new Error('offline') } })
    await submit(mount(Login, mountGlobals))
    expect(ElMessage.error).toHaveBeenCalledWith(i18n.global.t('common.networkError'))
    expect(auth.getToken()).toBeNull(); expect(testRouter.currentRoute.value.path).toBe('/login')
  })
})

describe('permission submit', () => {
  it.each([false, true])('submits validated fields to the appropriate endpoint (edit=%s)', async edit => {
    const list = { list: [{ id: 13, path: '/old', method: 'GET', description: 'old' }], total: 1 }
    const method = edit ? 'PUT' : 'POST'; const url = edit ? '/permissions/13' : '/permissions'
    install({ 'GET /permissions': list, [`${method} ${url}`]: {} })
    const w = mount(PermissionManagement, mountGlobals); await flushPromises(); await click(w, edit ? '编辑' : '新增')
    const dialog = w.find('[role=dialog]')
    await field(dialog, '/api/v1/users', '/api/v1/reports')
    dialog.findComponent(ElSelect).vm.$emit('update:modelValue', 'POST')
    await dialog.find('textarea').setValue('Report creation')
    await click(dialog, '确定')
    expect(sent(method, url)).toEqual([expect.objectContaining({ data: { path: '/api/v1/reports', method: 'POST', description: 'Report creation' } })])
    expect(sent('GET', '/permissions')).toHaveLength(2)
    expect(w.findAll('[role=dialog]')).toHaveLength(0)
    expect(ElMessage.success).toHaveBeenCalledWith(i18n.global.t(edit ? 'user.updateSuccess' : 'user.createSuccess'))
  })
})

describe('role actions', () => {
  it('editing preserves the readonly code and sends the modified description', async () => {
    install({ 'GET /roles': [role], 'PUT /roles/3': {} })
    const w = mount(RoleManagement, mountGlobals); await flushPromises(); await click(w, '编辑')
    const dialog = w.find('[role=dialog]')
    expect(dialog.findAll('input')[1]?.attributes('disabled')).toBeDefined()
    await dialog.find('textarea').setValue('Changed description'); await click(dialog, '确定')
    expect(sent('PUT', '/roles/3')[0]?.data).toEqual({ name: 'operator', code: 'operator', desc: 'Changed description' })
    expect(sent('GET', '/roles')).toHaveLength(2); expect(w.findAll('[role=dialog]')).toHaveLength(0)
  })
  it('permission assignment sends only checked methods', async () => {
    install({ 'GET /roles': [role], 'PUT /roles/3/permissions': {} })
    const w = mount(RoleManagement, mountGlobals); await flushPromises(); await click(w, '分配权限')
    const dialog = w.find('[role=dialog]')
    const switches = dialog.findAllComponents(ElSwitch)
    switches[0]!.vm.$emit('update:modelValue', true); switches[2]!.vm.$emit('update:modelValue', true)
    await click(dialog, '确定')
    expect(sent('PUT', '/roles/3/permissions')[0]?.data).toEqual({ permissions: [{ path: '/api/v1/users', method: 'GET' }, { path: '/api/v1/users', method: 'PUT' }] })
    expect(w.findAll('[role=dialog]')).toHaveLength(0)
    expect(ElMessage.success).toHaveBeenCalledWith(i18n.global.t('role.permAssigned'))
  })
})

describe.each([
  ['role', RoleManagement, '/roles', '/roles/3/status', [role]],
  ['user', UserManagement, '/users', '/users/7/status', { list: [{ id: 7, username: 'alice', status: 1, roles: [] }], total: 1 }],
] as const)('%s status', (_name, component, listUrl, statusUrl, data) => {
  it.each([false, true])('status action sends the target value and rolls back on failure=%s', async fail => {
    install({ [`GET ${listUrl}`]: structuredClone(data), [`PUT ${statusUrl}`]: () => { if (fail) throw new Error('unavailable'); return {} } })
    const w = mount(component, mountGlobals); await flushPromises()
    const toggle = w.findComponent(ElSwitch)
    await toggle.find('[role=switch]').trigger('click'); await flushPromises()
    expect(sent('PUT', statusUrl)[0]?.data).toEqual({ status: 0 })
    expect(toggle.props('modelValue')).toBe(fail ? 1 : 0)
    if (!fail) expect(ElMessage.success).toHaveBeenCalledWith(i18n.global.t('user.statusUpdated'))
    else expect(ElMessage.success).not.toHaveBeenCalled()
  })
})

describe('menu editing', () => {
  it('editing keeps menu type and component and refreshes the tree', async () => {
    install({ 'GET /menus/tree': [menu], 'PUT /menus/11': {} })
    const w = mount(MenuManagement, mountGlobals); await flushPromises(); await click(w, '编辑')
    const dialog = w.find('[role=dialog]'); await field(dialog, '请输入菜单名称', 'Updated dashboard'); await click(dialog, '确定')
    expect(sent('PUT', '/menus/11')[0]?.data).toEqual({ id: 11, parent_id: 0, name: 'Updated dashboard', path: '/dashboard', component: 'Dashboard', icon: 'Home', sort: 1, type: 2, perms: '', hidden: false })
    expect(sent('GET', '/menus/tree')).toHaveLength(2); expect(w.findAll('[role=dialog]')).toHaveLength(0)
  })
  it.each([false, true])('menu status waits for the server and preserves state on failure=%s', async fail => {
    install({ 'GET /menus/tree': [{ ...menu }], 'PUT /menus/11/status': () => { if (fail) throw new Error('offline'); return {} } })
    const w = mount(MenuManagement, mountGlobals); await flushPromises()
    const toggle = w.findComponent(ElSwitch)
    await toggle.trigger('click'); await flushPromises()
    expect(sent('PUT', '/menus/11/status')[0]?.data).toEqual({ status: 0 })
    expect(toggle.props('modelValue')).toBe(fail ? 1 : 0)
    if (!fail) expect(ElMessage.success).toHaveBeenCalledWith(i18n.global.t('menuMgmt.statusUpdated'))
  })
  it('nested menus retain hierarchy and button nodes are excluded from parent choices', async () => {
    install({ 'GET /menus/tree': [
      { ...menu, id: 20, name: 'Root directory', type: 'M', path: '', icon: '', children: [] },
      { ...menu, id: 21, parent_id: 20, name: 'Child menu', children: [] },
      { ...menu, id: 22, parent_id: 20, name: 'Button permission', type: 'F', component: '', path: '' },
      { ...menu, id: 23, parent_id: 999, name: 'Orphan' },
    ] })
    const w = mount(MenuManagement, mountGlobals); await flushPromises(); await click(w, '新增根菜单')
    const options = w.findComponent(ElTreeSelect).props('data')
    expect(options[0].children).toEqual([{ id: 20, name: 'Root directory', children: [{ id: 21, name: 'Child menu', children: undefined }] }])
    expect(w.text()).not.toContain('Orphan')
  })
})

describe('audit filtering', () => {
  it('date range expands to full days and reset clears all filters', async () => {
    install({ 'GET /audit/logs': { list: [], total: 0 } })
    const w = mount(OperationLog, mountGlobals); await flushPromises()
    w.findComponent(ElDatePicker).vm.$emit('update:modelValue', ['2026-01-01', '2026-01-02'])
    const selects = w.findAllComponents(ElSelect)
    selects[0]!.vm.$emit('update:modelValue', '用户'); selects[1]!.vm.$emit('update:modelValue', 2)
    await click(w, '搜索')
    expect(sent('GET', '/audit/logs').at(-1)?.params).toEqual({ username: '', module: '用户', status: 2, page: 1, page_size: 10, start: '2026-01-01 00:00:00', end: '2026-01-02 23:59:59' })
    await click(w, '重置')
    expect(sent('GET', '/audit/logs').at(-1)?.params).toEqual({ username: '', module: '', status: 0, page: 1, page_size: 10 })
  })
  it('query failure displays the audit error and permits retry', async () => {
    let fail = true
    install({ 'GET /audit/logs': () => { if (fail) throw new Error('offline'); return { list: [], total: 0 } } })
    const w = mount(OperationLog, mountGlobals); await flushPromises()
    expect(ElMessage.error).toHaveBeenCalledWith(i18n.global.t('audit.queryFailed'))
    fail = false; await click(w, '搜索'); expect(sent('GET', '/audit/logs')).toHaveLength(2)
    expect(w.text()).toContain('No Data')
  })
  it('failure details identify anonymous actors and show the server error', async () => {
    install({ 'GET /audit/logs': { list: [{ id: 7, username: '', module: '认证', operation: 'login', method: 'POST', path: '/login', ip: '', status: 2, error_msg: 'invalid credential', user_agent: '', trace_id: '', created_at: '' }], total: 1 } })
    const w = mount(OperationLog, mountGlobals); await flushPromises(); await click(w, '详情')
    expect(w.find('.el-drawer-stub').text()).toContain(i18n.global.t('audit.anonymous'))
    expect(w.find('.el-drawer-stub').text()).toContain('invalid credential')
    expect(w.find('.el-drawer-stub').text()).toContain(i18n.global.t('audit.failed'))
  })
})

describe('dashboard populated and failure states', () => {
  it('shows method/result variants, expands failure details, and opens the user list', async () => {
    const operations = ['GET', 'POST', 'PUT', 'DELETE', 'PATCH'].map((method, index) => ({ id: index + 1, username: 'actor', module: '用户', operation: 'change', method, path: '/users', ip: '192.0.2.1', status: index % 2 ? 2 : 1, error_msg: 'failure detail', created_at: '2026-01-01T00:00:00Z' }))
    install({ 'GET /dashboard/stats': { total_users: 10, today_new_users: 2, total_roles: 3, total_permissions: 4, total_menus: 5, today_operations: 6, today_logins: 7, today_failed_logins: 1, today_failed_ops: 2, recent_operations: operations, recent_failed_ops: [operations[1]] } })
    const warnings: string[] = []
    const w = mount(Dashboard, { global: { ...mountGlobals.global, config: { warnHandler: (message: string) => { warnings.push(message) } } } }); await flushPromises()
    expect(w.findAll('.kpi-alert')).toHaveLength(1)
    expect(w.text()).toContain('PATCH'); expect(w.text()).toContain('failure detail')
    await w.find('.sec-toggle').trigger('click'); await flushPromises()
    expect(w.find('.sec-detail').text()).toContain('192.0.2.1')
    await w.find('.kpi-card').trigger('click'); await flushPromises()
    expect(testRouter.currentRoute.value.path).toBe('/system/users')
    // The existing POST tag uses an empty type, rejected by this ElementPlus version.
    expect(warnings.every(message => message.includes('Invalid prop') && message.includes('type'))).toBe(true)
  })
  it('empty failure details are explained when expanded', async () => {
    install({ 'GET /dashboard/stats': { total_users: 0, recent_failed_ops: [], recent_operations: [] } })
    const w = mount(Dashboard, mountGlobals); await flushPromises(); await w.find('.sec-toggle').trigger('click')
    expect(w.find('.detail-empty').text()).toBe(i18n.global.t('dashboard.noDetail'))
  })
  it.each([Dashboard, MonitorDashboard])('load failures are surfaced and refresh remains available', async component => {
    const url = component === Dashboard ? '/dashboard/stats' : '/monitor/stats'
    install({ [`GET ${url}`]: () => { throw new Error('offline') } })
    const w = mount(component, mountGlobals); await flushPromises()
    expect(ElMessage.error).toHaveBeenCalledWith(i18n.global.t('dashboard.loadFailed'))
    await click(w, '刷新数据'); expect(sent('GET', url)).toHaveLength(2)
    expect(w.find('button').classes()).not.toContain('is-loading')
  })
  it('monitor formats long uptime, gigabytes and every HTTP method', async () => {
    install({ 'GET /monitor/stats': { uptime_seconds: 90060, memory_alloc_mb: 2048, memory_total_mb: 4096, http_metrics: ['GET', 'POST', 'PUT', 'DELETE'].map(method => ({ path: '/probe', method, status: 200, count: 3 })) } })
    const w = mount(MonitorDashboard, mountGlobals); await flushPromises()
    expect(w.text()).toContain('1d 1h 1m'); expect(w.text()).toContain('2.00 GB / 4.00 GB')
    expect(w.findAll('.el-tag').map(tag => tag.text())).toEqual(['GET', 'POST', 'PUT', 'DELETE'])
  })
})

describe('profile and sidebar fallback branches', () => {
  it('missing profile fields use empty values and the default timezone on save', async () => {
    auth.setToken('access'); install({ 'GET /user/profile': {}, 'PUT /user/profile': {} })
    const w = mount(ProfileDialog, { ...mountGlobals, props: { modelValue: false } })
    await w.setProps({ modelValue: true }); await flushPromises()
    expect(w.findAll('input').slice(0, 3).map(input => (input.element as HTMLInputElement).value)).toEqual(['', '', ''])
    expect(w.text()).toContain('Asia/Shanghai')
    await click(w, '确定')
    expect(sent('PUT', '/user/profile')[0]?.data).toEqual({ timezone: 'Asia/Shanghai' })
    expect(w.emitted('saved')).toEqual([[]])
  })
  it('cancel closes the profile without saving', async () => {
    install({})
    const w = mount(ProfileDialog, { ...mountGlobals, props: { modelValue: false } })
    await w.setProps({ modelValue: true }); await click(w, '取消')
    expect(calls).toEqual([]); expect(w.emitted('update:modelValue')).toContainEqual([false])
    expect(w.findAll('[role=dialog]')).toHaveLength(0)
  })
  it('nested sidebar menus open the active group and selection updates the route', async () => {
    await testRouter.replace('/system/users')
    install({ 'GET /menus': [{ id: 20, type: 'M', name: 'Custom directory', path: '/custom', children: [{ id: 21, path: '/system/users', name: 'Users', icon: 'Unknown' }, { id: 22, path: '/system/roles', name: 'Roles' }] }] })
    const w = mount(Sidebar, { ...mountGlobals, props: { collapse: false } }); await flushPromises()
    expect(w.find('.el-sub-menu').classes()).toContain('is-opened')
    const roles = w.findAll('.el-menu-item').find(item => item.text().includes('角色管理'))!
    await roles.trigger('click'); await flushPromises()
    expect(testRouter.currentRoute.value.path).toBe('/system/roles')
    expect(roles.classes()).toContain('is-active')
  })
})
