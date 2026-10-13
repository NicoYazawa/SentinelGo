import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import router from './index'
import { saveAuth, setPasswordChangeRequired } from '../utils/auth'

beforeEach(() => { localStorage.clear() })
afterEach(() => { localStorage.clear() })

describe('production route navigation', () => {
  it.each([
    ['/dashboard', 'Dashboard', '首页'],
    ['/system/users', 'UserManagement', '用户管理'],
    ['/system/roles', 'RoleManagement', '角色管理'],
    ['/system/menus', 'MenuManagement', '菜单管理'],
    ['/system/permissions', 'PermissionManagement', '权限管理'],
    ['/system/monitor', 'MonitorDashboard', '系统监控'],
    ['/system/audit', 'OperationLog', '操作日志'],
    ['/change-password', 'ChangePassword', '设置正式密码'],
  ])('authenticated navigation resolves %s to its page and title', async (path, component, title) => {
    saveAuth('access', 'refresh', 'admin')
    await router.push(path)
    expect(router.currentRoute.value.path).toBe(path)
    expect(document.title).toBe(title)
    expect(router.currentRoute.value.matched.at(-1)?.components?.default).toMatchObject({ __name: component })
  })
  it('unauthenticated deep links resolve login and retain the full destination', async () => {
    await router.push('/system/users?keyword=alice')
    expect(router.currentRoute.value.path).toBe('/login')
    expect(router.currentRoute.value.query.redirect).toBe('/system/users?keyword=alice')
    expect(router.currentRoute.value.matched.at(-1)?.components?.default).toMatchObject({ __name: 'Login' })
    expect(document.title).toBe('SentinelGo')
  })
  it('required password initialization redirects a protected deep link', async () => {
    saveAuth('access', 'refresh', 'admin'); setPasswordChangeRequired(true)
    await router.push('/system/users')
    expect(router.currentRoute.value.path).toBe('/change-password')
    expect(document.title).toBe('设置正式密码')
  })
  it('system index resolves the user page and unknown locations resolve the dashboard', async () => {
    saveAuth('access', 'refresh', 'admin')
    await router.push('/system')
    expect(router.currentRoute.value.path).toBe('/system/users')
    await router.push('/missing-location')
    expect(router.currentRoute.value.path).toBe('/dashboard')
  })
})
