/**
 * 操作审计日志 API 模块。
 */
import request from '@/utils/request'

/** 操作日志条目 */
export interface OperationLog {
  id: number
  user_id: number
  username: string
  module: string
  operation: string
  method: string
  path: string
  ip: string
  status: number
  error_msg: string
  user_agent: string
  trace_id: string
  created_at: string
}

/** 查询参数 */
export interface OpLogQuery {
  username?: string
  module?: string
  method?: string
  status?: number
  start?: string
  end?: string
  page?: number
  page_size?: number
}

/** 分页查询操作日志 */
export function getOperationLogs(params: OpLogQuery) {
  return request.get('/audit/logs', { params })
}
