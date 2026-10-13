/**
 * 监控 API 模块 — 获取系统运行时指标。
 *
 * MonitorStats 数据来源：
 *   - 运行时指标：uptime、goroutines、内存、CPU、Go 版本
 *   - HTTP 指标：Prometheus Counter + Histogram 采集的请求统计
 */
import request from '@/utils/request'

/** 单个 HTTP 方法的请求统计 */
export interface HTTPMetric {
  path: string
  method: string
  status: string
  count: number
}

/** 监控面板完整统计数据 */
export interface MonitorStats {
  uptime_seconds: number
  num_goroutines: number
  memory_alloc_mb: number
  memory_total_mb: number
  num_cpu: number
  go_version: string
  http_metrics: HTTPMetric[]
  http_total: number
  http_errors: number
  http_p99_seconds: number
}

/** 获取系统监控统计数据 */
export function getMonitorStats(): Promise<{ code: number; data: MonitorStats }> {
  return request.get('/monitor/stats')
}
