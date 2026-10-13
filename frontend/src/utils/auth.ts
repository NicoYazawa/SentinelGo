/**
 * 认证工具模块 — Token 和用户名的 localStorage 读写。
 *
 * 存储方案：
 *   - access_token  — JWT Access Token（15 分钟有效期）
 *   - refresh_token — JWT Refresh Token（7 天有效期）
 *   - username      — 当前用户名缓存（减少首屏请求）
 *
 * 关键函数：
 *   - isLoggedIn()    → 检查 Access Token 是否存在且有效
 *   - isRefreshable() → 检查 Refresh Token 是否存在（用于自动刷新）
 *   - saveAuth()      → 登录成功后一次性保存所有认证信息
 */

const ACCESS_KEY = 'access_token'
const REFRESH_KEY = 'refresh_token'
const USERNAME_KEY = 'username'

/** 获取 Access Token */
export function getToken(): string | null {
  return localStorage.getItem(ACCESS_KEY)
}

/** 设置 Access Token */
export function setToken(token: string): void {
  localStorage.setItem(ACCESS_KEY, token)
}

/** 获取 Refresh Token */
export function getRefreshToken(): string | null {
  return localStorage.getItem(REFRESH_KEY)
}

/** 设置 Refresh Token */
export function setRefreshToken(token: string): void {
  localStorage.setItem(REFRESH_KEY, token)
}

/** 清除所有 Token（退出登录时调用） */
export function removeToken(): void {
  localStorage.removeItem(ACCESS_KEY)
  localStorage.removeItem(REFRESH_KEY)
  localStorage.removeItem('password_change_required')
}

export function setPasswordChangeRequired(required: boolean): void {
  localStorage.setItem('password_change_required', String(required))
}

export function mustChangePassword(): boolean {
  return localStorage.getItem('password_change_required') === 'true'
}

/** 缓存当前用户名到 localStorage */
export function setUsername(name: string): void {
  localStorage.setItem(USERNAME_KEY, name)
}

/** 获取缓存的用户名 */
export function getUsername(): string | null {
  return localStorage.getItem(USERNAME_KEY)
}

/** 判断是否已登录（Access Token 非空且非无效值） */
export function isLoggedIn(): boolean {
  const token = getToken()
  return token !== null && token !== '' && token !== 'undefined' && token !== 'null'
}

/** 判断是否有可用的 Refresh Token */
export function isRefreshable(): boolean {
  const token = getRefreshToken()
  return token !== null && token !== '' && token !== 'undefined' && token !== 'null'
}

/** 登录成功后保存所有认证信息 */
export function saveAuth(accessToken: string, refreshToken: string, username: string): void {
  setToken(accessToken)
  setRefreshToken(refreshToken)
  setUsername(username)
}
