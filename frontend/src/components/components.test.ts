import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount, flushPromises, enableAutoUnmount } from '@vue/test-utils'
import { ElMessage } from 'element-plus'
import HelloWorld from './HelloWorld.vue'
import LanguageSwitcher from './LanguageSwitcher.vue'
import ProfileDialog from './ProfileDialog.vue'
import Sidebar from './Sidebar.vue'
import { mountGlobals } from '../test/mount'
import { mockApi, type ApiCall } from '../test/mockApi'
import * as auth from '../utils/auth'
import { currentLang, setLocale, i18n } from '../locales'

enableAutoUnmount(afterEach)
let calls: ApiCall[]
let unexpected: string[]
const profile = { nickname: 'Alice', email: 'alice@example.com', phone: '13800000000', timezone: 'Europe/London' }
function apiData(url: string, config: any) {
  const route = `${config.method.toUpperCase()} ${url}`
  switch (route) {
    case 'GET /user/profile': return { code: 0, data: profile }
    case 'PUT /user/profile': return { code: 0, data: null }
    case 'GET /menus': return { code: 0, data: [] }
    default: unexpected.push(route); throw new Error(`Unexpected request: ${route}`)
  }
}
beforeEach(() => {
  localStorage.clear()
  setLocale('zh-CN')
  unexpected = []
  calls = mockApi(apiData)
  for (const method of ['success', 'error'] as const) vi.spyOn(ElMessage, method).mockImplementation(() => undefined as any)
})
afterEach(() => {
  vi.restoreAllMocks()
  expect(unexpected).toEqual([])
})
async function openProfile() {
  const w = mount(ProfileDialog, { ...mountGlobals, props: { modelValue: false } })
  await w.setProps({ modelValue: true })
  await flushPromises()
  return w
}
async function save(w: any) {
  const button = w.findAll('button').find((b: any) => b.text().includes('确定'))
  if (!button) throw new Error('保存按钮未找到')
  await button.trigger('click')
  await flushPromises()
}
describe('HelloWorld', () => {
  it('点击计数器自增', async () => {
    const w = mount(HelloWorld, mountGlobals)
    expect(w.text()).toContain('Count is 0')
    await w.find('button.counter').trigger('click')
    expect(w.text()).toContain('Count is 1')
  })
})
describe('LanguageSwitcher', () => {
  it('未登录时切换语言不调接口', async () => {
    const w = mount(LanguageSwitcher, mountGlobals)
    await w.find('select').setValue('en-US'); await flushPromises()
    expect(currentLang.value).toBe('en-US'); expect(calls).toEqual([])
  })
  it('已登录时提交新的语言偏好', async () => {
    auth.setToken('x')
    const w = mount(LanguageSwitcher, mountGlobals)
    await w.find('select').setValue('en-US'); await flushPromises()
    expect(currentLang.value).toBe('en-US')
    expect(calls).toEqual([expect.objectContaining({ method: 'PUT', url: '/user/profile', data: { locale: 'en-US' }, authorization: 'Bearer x' })])
  })
})
describe('ProfileDialog', () => {
  it('打开时无 token 则不加载个人资料', async () => {
    await openProfile(); expect(calls).toEqual([])
  })
  it('打开时加载个人资料并显示接口字段', async () => {
    auth.setToken('x')
    const w = await openProfile()
    expect(calls).toEqual([expect.objectContaining({ method: 'GET', url: '/user/profile' })])
    expect(w.findAll('input').map(i => (i.element as HTMLInputElement).value)).toEqual(expect.arrayContaining(['Alice', 'alice@example.com', '13800000000']))
    expect(w.text()).toContain('Europe/London')
    expect(localStorage.getItem('user_timezone')).toBe('Europe/London')
  })
  it('加载失败时请求确实发生并显示失败提示', async () => {
    auth.setToken('x'); calls = mockApi(() => ({ code: 50001, msg: 'load-failed' }))
    await openProfile()
    expect(calls).toEqual([expect.objectContaining({ method: 'GET', url: '/user/profile' })])
    expect(ElMessage.error).toHaveBeenCalledWith(i18n.global.t('profile.loadFailed'))
  })
  it('保存输入字段成功，通知父组件并关闭', async () => {
    auth.setToken('x')
    const w = await openProfile()
    await w.findAll('input')[0]!.setValue('Updated Alice'); await save(w)
    expect(calls.filter(c => c.method === 'PUT')).toEqual([expect.objectContaining({ url: '/user/profile', data: { ...profile, nickname: 'Updated Alice' } })])
    expect(w.emitted('saved')).toEqual([[]])
    expect(w.emitted('update:modelValue')).toContainEqual([false])
    expect(w.find('.el-dialog-stub').exists()).toBe(false)
  })
  it('保存失败时提示，保留弹窗且不发 saved 事件', async () => {
    auth.setToken('x')
    calls = mockApi((url, config) => config.method === 'put' ? { code: 50001, msg: 'save-failed' } : apiData(url, config))
    const w = await openProfile(); await save(w)
    expect(calls.filter(c => c.method === 'PUT')).toHaveLength(1)
    expect(ElMessage.error).toHaveBeenCalledWith('save-failed')
    expect(w.emitted('saved')).toBeUndefined()
    expect(w.emitted('update:modelValue') || []).not.toContainEqual([false])
    expect(w.find('.el-dialog-stub').exists()).toBe(true)
  })
})
describe('Sidebar', () => {
  it('显示接口独有的菜单，避免默认菜单造成假阳性', async () => {
    calls = mockApi((url, config) => {
      if (url === '/menus' && config.method === 'get') return { code: 0, data: [{ id: 42, parent_id: 0, name: '接口专属菜单', path: '/custom', icon: 'HomeFilled', sort: 1, type: 'C', status: 1, hidden: false }] }
      return apiData(url, config)
    })
    const w = mount(Sidebar, { ...mountGlobals, props: { collapse: false } }); await flushPromises()
    expect(calls).toEqual([expect.objectContaining({ method: 'GET', url: '/menus' })])
    expect(w.text()).toContain('接口专属菜单')
  })
  it('接口失败后显示默认菜单', async () => {
    calls = mockApi(() => ({ code: 50001, msg: 'menus-failed' }))
    const w = mount(Sidebar, { ...mountGlobals, props: { collapse: false } }); await flushPromises()
    expect(calls).toEqual([expect.objectContaining({ method: 'GET', url: '/menus' })])
    expect(w.text()).toContain('用户管理'); expect(w.text()).not.toContain('接口专属菜单')
  })
})
