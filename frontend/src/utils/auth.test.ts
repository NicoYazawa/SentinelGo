import { afterEach, describe, expect, it } from 'vitest'
import * as adminAuth from './auth'
import * as userAuth from '@user/utils/auth'

// 两套前端 auth 实现核心一致（仅 user 端 key 带 user_ 前缀，且少了 isRefreshable），
// 对共有 API 用同一组用例覆盖两者。
const suites: Array<[string, typeof adminAuth]> = [
  ['admin', adminAuth],
  ['user', userAuth],
]

describe.each(suites)('auth utils (%s)', (_name, auth) => {
  afterEach(() => {
    localStorage.clear()
  })

  it('setToken / getToken 可正常存取', () => {
    expect(auth.getToken()).toBeNull()
    auth.setToken('abc')
    expect(auth.getToken()).toBe('abc')
  })

  it('setRefreshToken / getRefreshToken 可正常存取', () => {
    expect(auth.getRefreshToken()).toBeNull()
    auth.setRefreshToken('rt')
    expect(auth.getRefreshToken()).toBe('rt')
  })

  it('saveAuth 一次性写入 token 与用户名', () => {
    auth.saveAuth('a', 'r', 'neo')
    expect(auth.getToken()).toBe('a')
    expect(auth.getRefreshToken()).toBe('r')
    expect(auth.getUsername()).toBe('neo')
  })

  it('removeToken 清除 token', () => {
    auth.saveAuth('a', 'r', 'neo')
    auth.removeToken()
    expect(auth.getToken()).toBeNull()
    expect(auth.getRefreshToken()).toBeNull()
  })

  it('isLoggedIn 对空值 / 字面量 "undefined" / "null" 返回 false', () => {
    expect(auth.isLoggedIn()).toBe(false)
    auth.setToken('')
    expect(auth.isLoggedIn()).toBe(false)
    auth.setToken('undefined')
    expect(auth.isLoggedIn()).toBe(false)
    auth.setToken('null')
    expect(auth.isLoggedIn()).toBe(false)
    auth.setToken('valid')
    expect(auth.isLoggedIn()).toBe(true)
  })

  it('mustChangePassword 读取 password_change_required 标记', () => {
    expect(auth.mustChangePassword()).toBe(false)
    auth.setPasswordChangeRequired(true)
    expect(auth.mustChangePassword()).toBe(true)
  })

  it('removeToken 清除 password_change_required 标记（仅 admin 端校验 key）', () => {
    if (_name !== 'admin') return
    auth.setPasswordChangeRequired(true)
    auth.removeToken()
    expect(localStorage.getItem('password_change_required')).toBeNull()
  })
})

// isRefreshable 仅 admin 端实现，单独覆盖。
describe('auth utils (admin) extra', () => {
  afterEach(() => localStorage.clear())

  it('isRefreshable 仅在 refresh_token 有效时为 true', () => {
    expect(adminAuth.isRefreshable()).toBe(false)
    adminAuth.setRefreshToken('r')
    expect(adminAuth.isRefreshable()).toBe(true)
    adminAuth.setRefreshToken('null')
    expect(adminAuth.isRefreshable()).toBe(false)
  })
})
