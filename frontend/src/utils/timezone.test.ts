import { describe, it, expect, beforeEach } from 'vitest'
import { getUserTimezone, setUserTimezone } from './timezone'

describe('timezone utils', () => {
  beforeEach(() => {
    localStorage.clear()
    setUserTimezone('Asia/Shanghai')
  })

  it('getUserTimezone 返回当前用户时区', () => {
    expect(getUserTimezone()).toBe('Asia/Shanghai')
    setUserTimezone('Europe/London')
    expect(getUserTimezone()).toBe('Europe/London')
  })

  it('setUserTimezone 持久化到 localStorage', () => {
    setUserTimezone('America/New_York')
    expect(localStorage.getItem('user_timezone')).toBe('America/New_York')
    expect(getUserTimezone()).toBe('America/New_York')
  })

  it('setUserTimezone 空值被忽略（保持不变）', () => {
    setUserTimezone('Asia/Shanghai')
    setUserTimezone('')
    expect(getUserTimezone()).toBe('Asia/Shanghai')
  })

  it('getUserTimezone 兜底 Asia/Shanghai', () => {
    setUserTimezone('')
    expect(getUserTimezone()).toBe('Asia/Shanghai')
  })
})
