import { describe, it, expect, vi, beforeEach } from 'vitest'
import { routeGuard } from './index'

function makeTo(meta: Record<string, unknown>, path: string, fullPath = path) {
  return { meta, path, fullPath }
}

describe('routeGuard', () => {
  beforeEach(() => {
    localStorage.clear()
    document.title = 'SentinelGo'
  })

  it('需要认证但未登录 → 跳转登录页并携带 redirect', () => {
    const next = vi.fn()
    routeGuard(makeTo({ requiresAuth: true }, '/dashboard'), {}, next)
    expect(next).toHaveBeenCalledWith('/login?redirect=' + encodeURIComponent('/dashboard'))
  })

  it('已登录且无需改密 → 放行', () => {
    localStorage.setItem('access_token', 'x')
    const next = vi.fn()
    routeGuard(makeTo({ requiresAuth: true }, '/dashboard'), {}, next)
    expect(next).toHaveBeenCalledWith()
  })

  it('已登录但需改密且非改密页 → 跳转 /change-password', () => {
    localStorage.setItem('access_token', 'x')
    localStorage.setItem('password_change_required', 'true')
    const next = vi.fn()
    routeGuard(makeTo({ requiresAuth: true }, '/dashboard'), {}, next)
    expect(next).toHaveBeenCalledWith('/change-password')
  })

  it('已登录访问 /login → 跳转首页', () => {
    localStorage.setItem('access_token', 'x')
    const next = vi.fn()
    routeGuard(makeTo({ requiresAuth: false }, '/login'), {}, next)
    expect(next).toHaveBeenCalledWith('/')
  })

  it('公开路由（requiresAuth:false）未登录 → 放行', () => {
    const next = vi.fn()
    routeGuard(makeTo({ requiresAuth: false }, '/login'), {}, next)
    expect(next).toHaveBeenCalledWith()
  })

  it('设置页面标题', () => {
    localStorage.setItem('access_token', 'x')
    const next = vi.fn()
    routeGuard(makeTo({ requiresAuth: true, title: '首页' }, '/dashboard'), {}, next)
    expect(document.title).toBe('首页')
  })
})
