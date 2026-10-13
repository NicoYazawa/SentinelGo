// 时间格式化工具：将后端返回的 ISO8601 时间字符串（含时区，如 2026-10-10T01:23:45Z）
// 按【当前用户个人时区】显示（见 @/utils/timezone）。后端统一以 UTC 存储/返回，
// 国内用户默认 Asia/Shanghai（北京时间），用户在「个人中心」可改为任意 IANA 时区。
// 这样同一份 UTC 数据，各地用户看到的是各自本地时间。

import { getUserTimezone } from './timezone'

export function formatDateTime(input?: string | null): string {
  if (!input) return '-'
  const dt = new Date(input)
  if (Number.isNaN(dt.getTime())) return input

  const tz = getUserTimezone()
  try {
    const parts = new Intl.DateTimeFormat('zh-CN', {
      timeZone: tz,
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: false,
    }).formatToParts(dt)
    const g = (t: string) => parts.find((p) => p.type === t)?.value ?? ''
    return `${g('year')}-${g('month')}-${g('day')} ${g('hour')}:${g('minute')}:${g('second')}`
  } catch {
    // 时区非法时回退到浏览器本地时区
    return dt.toLocaleString('zh-CN', { hour12: false })
  }
}
