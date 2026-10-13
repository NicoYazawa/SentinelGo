import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import axios from 'axios'
const messageError = vi.hoisted(() => vi.fn())

// 拦截器仅用到 ElMessage.error，mock 掉避免依赖 Vue 运行时上下文。
vi.mock('element-plus', () => ({
  ElMessage: { error: messageError },
}))
vi.mock('../../../user-frontend/node_modules/element-plus', () => ({
  ElMessage: { error: messageError },
}))

import adminRequest from './request'
import userRequest from '@user/utils/request'
import * as adminAuth from './auth'
import * as userAuth from '@user/utils/auth'

// 两套 request 实例共享全局 axios 用于 refresh；用同一组用例覆盖两者。
// pwdKey 为各端 setPasswordChangeRequired 使用的 localStorage 键名（user 端带 user_ 前缀）。
const suites: Array<[string, typeof adminRequest, typeof adminAuth, string]> = [
  ['admin', adminRequest, adminAuth, 'password_change_required'],
  ['user', userRequest, userAuth, 'user_password_change_required'],
]

type AdapterHandler = (url: string, config: any) => any

// 给 request 实例与全局 axios（refresh 走全局 axios.post）同时挂载可控 adapter。
function installAdapter(request: typeof adminRequest, handler: AdapterHandler) {
  const adapter = async (config: any) =>
    ({
      data: await handler(config.url || '', config),
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    })
  request.defaults.adapter = adapter
  axios.defaults.adapter = adapter
}

describe.each(suites)('request interceptor (%s)', (_name, request, auth, pwdKey) => {
  beforeEach(() => {
    localStorage.clear()
    vi.clearAllMocks()
    // jsdom 下 window.location.href 赋值会触发导航告警且只读，替换为可写桩对象。
    Object.defineProperty(window, 'location', {
      writable: true,
      configurable: true,
      value: { pathname: '/dashboard', href: '' },
    })
  })

  afterEach(() => {
    auth.removeToken()
    localStorage.clear()
  })

  it('code === 0 时直接返回业务数据', async () => {
    installAdapter(request, () => ({ code: 0, data: { ok: true } }))
    const res = await request.get('/user/info')
    expect(res.code).toBe(0)
    expect(res.data).toEqual({ ok: true })
  })

  it('code === 20003 设置强制改密标记并跳转到 /change-password', async () => {
    installAdapter(request, () => ({ code: 20003, msg: 'please set password' }))
    await expect(request.get('/user/info')).rejects.toThrow('please set password')
    expect(localStorage.getItem(pwdKey)).toBe('true')
    expect(window.location.href).toBe('/change-password')
  })

  it('token 过期(20001)时并发请求只触发一次 refresh 且全部成功', async () => {
    let refreshCalls = 0
    let releaseRefresh!: () => void
    const refreshGate = new Promise<void>(resolve => { releaseRefresh = resolve })
    const attempts = new Map<string, string[]>()
    let expiredResponses = 0
    installAdapter(request, async (url, config) => {
      if (url.includes('/auth/refresh')) {
        refreshCalls++
        expect(JSON.parse(config.data)).toEqual({ refresh_token: 'oldR' })
        await refreshGate
        return { code: 0, data: { access_token: 'newA', refresh_token: 'newR' } }
      }
      expect(['/user/info', '/user/profile']).toContain(url)
      const history = attempts.get(url) || []
      history.push(config.headers.Authorization)
      attempts.set(url, history)
      if (history.length === 1) {
        expiredResponses++
        return { code: 20001, msg: 'expired' }
      }
      expect(history).toEqual(['Bearer oldA', 'Bearer newA'])
      return { code: 0, data: { ok: true } }
    })
    auth.saveAuth('oldA', 'oldR', 'neo')

    const pending = Promise.all([request.get('/user/info'), request.get('/user/profile')])
    let coordinationError: unknown
    try {
      await vi.waitFor(() => {
        expect(expiredResponses).toBe(2)
        expect(refreshCalls).toBe(1)
      })
    } catch (error) {
      coordinationError = error
    } finally {
      releaseRefresh()
    }
    const [a, b] = await pending
    if (coordinationError) throw coordinationError
    expect(a.code).toBe(0)
    expect(b.code).toBe(0)
    expect(refreshCalls).toBe(1)
    expect(attempts.get('/user/info')).toEqual(['Bearer oldA', 'Bearer newA'])
    expect(attempts.get('/user/profile')).toEqual(['Bearer oldA', 'Bearer newA'])
    expect(auth.getToken()).toBe('newA')
    expect(auth.getRefreshToken()).toBe('newR')
  })

  it('refresh 失败时清除 token 并跳转登录页', async () => {
    installAdapter(request, (url) => {
      if (url.includes('/auth/refresh')) {
        return { code: 50001, msg: 'refresh failed' }
      }
      return { code: 20001, msg: 'expired' }
    })
    auth.saveAuth('oldA', 'oldR', 'neo')

    await expect(request.get('/user/info')).rejects.toThrow()
    expect(auth.getToken()).toBeNull()
    expect(auth.getRefreshToken()).toBeNull()
    expect(window.location.href).toContain('/login')
  })

  it.each([20000, 20001])('without refresh credentials code %s expires the session', async code => {
    auth.saveAuth('access', '', 'neo')
    installAdapter(request, () => ({ code }))
    await expect(request.get('/private')).rejects.toThrow()
    expect(auth.getToken()).toBeNull()
    expect(window.location.href).toBe('/login?redirect=%2Fdashboard')
  })

  it('a rejected retry does not refresh indefinitely', async () => {
    auth.saveAuth('access', 'refresh', 'neo')
    const urls: string[] = []
    installAdapter(request, url => {
      urls.push(url)
      return url.includes('/auth/refresh')
        ? { code: 0, data: { access_token: 'new', refresh_token: 'new-refresh' } }
        : { code: 20001, msg: 'still expired' }
    })
    await expect(request.get('/private')).rejects.toThrow('still expired')
    expect(urls).toEqual(['/private', '/api/v1/auth/refresh', '/private'])
    expect(auth.getToken()).toBeNull()
    expect(window.location.href).toBe('/login?redirect=%2Fdashboard')
  })

  it.each(['denied', undefined])('public auth errors display a message without refreshing (%s)', async msg => {
    auth.saveAuth('access', 'refresh', 'neo')
    const urls: string[] = []
    installAdapter(request, url => { urls.push(url); return { code: 20000, msg } })
    await expect(request.post('/auth/login')).rejects.toThrow()
    expect(urls).toEqual(['/auth/login'])
    expect(messageError).toHaveBeenCalledWith(msg || expect.any(String))
    expect(auth.getToken()).toBe('access')
    expect(window.location.href).toBe('')
  })

  it('password initialization errors preserve the current initialization page', async () => {
    window.location.pathname = '/change-password'
    installAdapter(request, () => ({ code: 20003 }))
    await expect(request.get('/private')).rejects.toThrow()
    expect(localStorage.getItem(pwdKey)).toBe('true')
    expect(window.location.href).toBe('')
  })

  it.each([401, 503])('HTTP %s rejects the original error and applies the appropriate session effect', async status => {
    window.location.pathname = '/login'
    auth.saveAuth('access', 'refresh', 'neo')
    const error = { response: { status }, message: 'server unavailable' }
    request.defaults.adapter = async () => { throw error }
    await expect(request.get('/private')).rejects.toBe(error)
    expect(window.location.href).toBe('')
    expect(auth.getToken()).toBe(status === 401 ? null : 'access')
    if (status === 503) expect(messageError).toHaveBeenCalledWith('server unavailable')
    else expect(messageError).not.toHaveBeenCalled()
  })

  it('network errors without a message display the fallback', async () => {
    const error = {}
    request.defaults.adapter = async () => { throw error }
    await expect(request.get('/private')).rejects.toBe(error)
    expect(messageError).toHaveBeenCalledWith(expect.any(String))
  })
})
