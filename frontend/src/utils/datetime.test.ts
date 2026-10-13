import { describe, it, expect, beforeEach } from 'vitest'
import { formatDateTime } from './datetime'
import { setUserTimezone } from './timezone'

describe('formatDateTime', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('空值返回占位符 -', () => {
    expect(formatDateTime(undefined)).toBe('-')
    expect(formatDateTime(null)).toBe('-')
    expect(formatDateTime('')).toBe('-')
  })

  it('非法时间字符串原样返回', () => {
    expect(formatDateTime('not-a-date')).toBe('not-a-date')
  })

  it('按用户时区格式化（Asia/Shanghai, UTC+8）', () => {
    setUserTimezone('Asia/Shanghai')
    // 2026-10-10T01:23:45Z -> 北京时间 2026-10-10 09:23:45
    expect(formatDateTime('2026-10-10T01:23:45Z')).toBe('2026-10-10 09:23:45')
  })

  it('按用户时区格式化（America/New_York, UTC-4 夏令时）', () => {
    setUserTimezone('America/New_York')
    // 2026-10-10T01:23:45Z -> 纽约 2026-10-09 21:23:45 (EDT, UTC-4)
    expect(formatDateTime('2026-10-10T01:23:45Z')).toBe('2026-10-09 21:23:45')
  })

  it('非法时区回退到本地时区（返回字符串而非抛错）', () => {
    setUserTimezone('Invalid/Zone')
    const out = formatDateTime('2026-10-10T01:23:45Z')
    expect(typeof out).toBe('string')
    expect(out.length).toBeGreaterThan(0)
  })
})
