// 个人时区状态管理：保存当前登录用户设置的时区（IANA，如 Asia/Shanghai）。
// 持久化到 localStorage，刷新后无需重新请求即可用于时间格式化。
// 默认 Asia/Shanghai（契合产品面向中国用户），用户在「个人中心」可修改。
import { ref } from 'vue'

const KEY = 'user_timezone'

const stored = localStorage.getItem(KEY)
export const userTimezone = ref<string>(stored || 'Asia/Shanghai')

/** 获取当前用户时区（兜底 Asia/Shanghai） */
export function getUserTimezone(): string {
  return userTimezone.value || 'Asia/Shanghai'
}

/** 设置并持久化用户时区 */
export function setUserTimezone(tz: string): void {
  if (!tz) return
  userTimezone.value = tz
  localStorage.setItem(KEY, tz)
}
