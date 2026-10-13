const ACCESS_KEY = 'user_access_token'
const REFRESH_KEY = 'user_refresh_token'
const USERNAME_KEY = 'user_username'

export function getToken(): string | null {
  return localStorage.getItem(ACCESS_KEY)
}

export function setToken(token: string): void {
  localStorage.setItem(ACCESS_KEY, token)
}

export function getRefreshToken(): string | null {
  return localStorage.getItem(REFRESH_KEY)
}

export function setRefreshToken(token: string): void {
  localStorage.setItem(REFRESH_KEY, token)
}

export function removeToken(): void {
  localStorage.removeItem(ACCESS_KEY)
  localStorage.removeItem(REFRESH_KEY)
  localStorage.removeItem('user_password_change_required')
}

export function setPasswordChangeRequired(required: boolean): void {
  localStorage.setItem('user_password_change_required', String(required))
}

export function mustChangePassword(): boolean {
  return localStorage.getItem('user_password_change_required') === 'true'
}

export function setUsername(name: string): void {
  localStorage.setItem(USERNAME_KEY, name)
}

export function getUsername(): string | null {
  return localStorage.getItem(USERNAME_KEY)
}

export function isLoggedIn(): boolean {
  const token = getToken()
  return token !== null && token !== '' && token !== 'undefined' && token !== 'null'
}

export function saveAuth(accessToken: string, refreshToken: string, username: string): void {
  setToken(accessToken)
  setRefreshToken(refreshToken)
  setUsername(username)
}
