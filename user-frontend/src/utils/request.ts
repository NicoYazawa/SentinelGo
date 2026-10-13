import axios from 'axios'
import type { InternalAxiosRequestConfig } from 'axios'
import { ElMessage } from 'element-plus'
import { getToken, removeToken, getRefreshToken, saveAuth, getUsername, setPasswordChangeRequired } from './auth'
import { i18n, currentLang } from '../locales'

const request = axios.create({ baseURL: '/api/v1', timeout: 10000 })
let refreshing: Promise<string> | null = null
const publicAuth = /^\/auth\/(login|register|refresh|sms\/(login|send)|password\/(forgot|reset)|oauth\/)/
type RetryConfig = InternalAxiosRequestConfig & { authRetried?: boolean }

request.interceptors.request.use(config => {
  const access = getToken()
  if (access) config.headers.Authorization = `Bearer ${access}`
  config.headers['Accept-Language'] = currentLang.value
  return config
})

request.interceptors.response.use(async response => {
  const res = response.data
  if (res.code === 0) return res
  if (res.code === 20003) {
    setPasswordChangeRequired(true)
    if (window.location.pathname !== '/change-password') window.location.href = '/change-password'
    throw new Error(res.msg || i18n.global.t('common.tip'))
  }
  const config = response.config as RetryConfig
  if ((res.code === 20000 || res.code === 20001) && !publicAuth.test(config.url || '')) {
    if (config.authRetried || !getRefreshToken()) {
      removeToken()
      redirectToLogin()
      throw new Error(res.msg || i18n.global.t('common.loginExpired'))
    }
    config.authRetried = true
    try {
      const access = await refreshAccessToken()
      config.headers.Authorization = `Bearer ${access}`
    } catch {
      removeToken()
      redirectToLogin()
      throw new Error(i18n.global.t('common.loginExpired'))
    }
    return request(config)
  }
  ElMessage.error(res.msg || i18n.global.t('common.requestFailed'))
  throw new Error(res.msg || i18n.global.t('common.requestFailed'))
}, error => {
  if (error.response?.status === 401) {
    removeToken()
    redirectToLogin()
  } else {
    ElMessage.error(error.message || i18n.global.t('common.networkError'))
  }
  return Promise.reject(error)
})

function refreshAccessToken(): Promise<string> {
  if (!refreshing) {
    refreshing = axios.post('/api/v1/auth/refresh', { refresh_token: getRefreshToken() }, { timeout: 10000 })
      .then(response => {
        if (response.data.code !== 0) throw new Error(i18n.global.t('common.requestFailed'))
        const { access_token, refresh_token } = response.data.data
        saveAuth(access_token, refresh_token, getUsername() || '')
        return access_token as string
      }).finally(() => { refreshing = null })
  }
  return refreshing
}

function redirectToLogin() {
  const current = window.location.pathname
  if (current !== '/login') window.location.href = `/login?redirect=${encodeURIComponent(current)}`
}

export default request
