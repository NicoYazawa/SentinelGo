import { describe, it, expect, beforeEach } from 'vitest'
import { setLocale, elementLocale, i18n, currentLang } from './index'

describe('locales', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('elementLocale 按语言返回不同语言包', () => {
    const zh = elementLocale('zh-CN')
    const en = elementLocale('en-US')
    expect(zh).toBeDefined()
    expect(en).toBeDefined()
    expect(zh).not.toBe(en)
  })

  it('setLocale 更新 currentLang 并持久化', () => {
    setLocale('en-US')
    expect(currentLang.value).toBe('en-US')
    expect(localStorage.getItem('user_locale')).toBe('en-US')
    expect(document.documentElement.lang).toBe('en-US')
  })

  it('setLocale 忽略不支持的语言', () => {
    setLocale('zh-CN')
    setLocale('fr-FR' as any)
    expect(currentLang.value).toBe('zh-CN')
  })

  it('i18n 翻译生效（中英文切换）', () => {
    setLocale('zh-CN')
    expect(i18n.global.t('common.requestFailed')).toBe('请求失败')
    setLocale('en-US')
    expect(i18n.global.t('common.requestFailed')).toBe('Request failed')
  })
})
