/**
 * 首页仪表盘 API 模块 — 聚合统计（KPI 卡片 + 安全与审计看板）。
 */
import request from '@/utils/request'

/** 最近操作 / 异常操作条目（复用审计日志结构） */
export interface OpItem {
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
  created_at: string
}

/** 仪表盘聚合数据 */
export interface DashboardStats {
  total_users: number
  today_new_users: number
  total_roles: number
  total_permissions: number
  total_menus: number
  today_operations: number
  today_failed_ops: number
  today_logins: number
  today_failed_logins: number
  recent_operations: OpItem[]
  recent_failed_ops: OpItem[]
}

/** 获取首页仪表盘统计 */
export function getDashboardStats(): Promise<{ code: number; data: DashboardStats; msg: string }> {
  return request.get('/dashboard/stats')
}
