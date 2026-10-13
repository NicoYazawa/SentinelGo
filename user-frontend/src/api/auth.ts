import request from '@/utils/request'

export interface LoginResult {
  user_id: number
  username: string
  access_token: string
  refresh_token: string
  expires_in: number
  token_type: string
  password_change_required: boolean
}

export interface ChannelInfo {
  id: number
  user_id: number
  identity_type: string
  identifier: string
}

export function loginByPassword(username: string, password: string) {
  return request.post<any, { code: number; data: LoginResult; msg: string }>('/auth/login', { username, password })
}

export function register(data: { username: string; password: string; nickname?: string; email?: string; phone?: string }) {
  return request.post<any, { code: number; data: { user_id: number; username: string }; msg: string }>('/auth/register', data)
}

export function sendSMS(phone: string, scene: 'login' | 'bind' = 'login') {
  // dev_code 仅开发模式（SMS provider=dev）由后端返回，生产环境为 undefined
  return request.post<any, { code: number; data: { msg: string; dev_code?: string }; msg: string }>('/auth/sms/send', { phone, scene })
}

export function loginBySMS(phone: string, code: string) {
  return request.post<any, { code: number; data: LoginResult; msg: string }>('/auth/sms/login', { phone, code })
}

export function logout() {
  return request.post('/auth/logout')
}

export function getChannels() {
  return request.get<any, { code: number; data: ChannelInfo[]; msg: string }>('/auth/channels')
}

export function bindPhone(phone: string, code: string) {
	return request.post<unknown, { code: number; data: unknown; msg: string }>('/auth/bind/phone', { phone, code })
}

export function unbindChannel(identityType: string) {
	return request.post<unknown, { code: number; data: unknown; msg: string }>('/auth/unbind', { identity_type: identityType })
}

export function forgotPassword(phone: string) {
  return request.post('/auth/password/forgot', { phone })
}

export function resetPassword(phone: string, code: string, newPassword: string) {
	return request.post<unknown, { code: number; data: unknown; msg: string }>('/auth/password/reset', { phone, code, new_password: newPassword })
}

export function getProfile() {
  return request.get<any, { code: number; data: { id: number; username: string; nickname: string; email: string; phone: string; avatar: string }; msg: string }>('/user/profile')
}

export function updateProfile(nickname: string, email: string, phone: string) {
	return request.put<unknown, { code: number; data: unknown; msg: string }>('/user/profile', { nickname, email, phone })
}

export function uploadAvatar(file: File) {
  const formData = new FormData()
  formData.append('avatar', file)
  return request.post<any, { code: number; data: { avatar_url: string }; msg: string }>('/user/avatar', formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
}
