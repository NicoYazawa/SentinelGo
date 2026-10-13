// 通过 axios adapter 拦截所有请求，返回可控业务数据，避免真实网络。
// handler 接收 (url, config) 返回「业务响应体」{ code, data, msg }；
// 非 0 code 会触发 request 拦截器抛错（从而走组件的 catch 分支）。
import request from '@/utils/request'
import axios from 'axios'

export interface ApiCall {
  method: string
  url: string
  data: any
  params: any
  authorization: string | undefined
}

export function mockApi(handler: (url: string, config?: any) => any): ApiCall[] {
  const calls: ApiCall[] = []
  const adapter = async (config: any) => {
    calls.push({
      method: (config.method || 'get').toUpperCase(),
      url: config.url || '',
      data: typeof config.data === 'string' ? JSON.parse(config.data) : config.data,
      params: config.params,
      authorization: config.headers?.Authorization,
    })
    return {
      data: await handler(config.url || '', config),
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    }
  }
  ;(request as any).defaults.adapter = adapter
  ;(axios as any).defaults.adapter = adapter
  return calls
}
