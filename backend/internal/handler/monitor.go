package handler

import (
	"runtime"
	"time"

	"sentinelgo/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

// startTime 记录服务启动时间，用于计算 uptime。
var startTime = time.Now()

// monitorGatherer 是监控指标采集源，默认使用 Prometheus 全局采集器。
// 测试时可替换为自定义 Registry 以覆盖 HTTP 指标相关分支。
var monitorGatherer = prometheus.DefaultGatherer

// MonitorHandler 是系统监控的 HTTP 控制器，提供运行时状态查询。
type MonitorHandler struct{}

// NewMonitorHandler 创建监控 Handler。
func NewMonitorHandler() *MonitorHandler {
	return &MonitorHandler{}
}

// MonitorStats 是监控统计数据结构，包含运行时指标和 HTTP 指标。
type MonitorStats struct {
	UptimeSeconds  int64        `json:"uptime_seconds"`   // 服务运行时长（秒）
	NumGoroutines  int          `json:"num_goroutines"`   // 当前 goroutine 数量
	MemoryAllocMB  float64      `json:"memory_alloc_mb"`  // 已分配堆内存（MB）
	MemoryTotalMB  float64      `json:"memory_total_mb"`  // 系统已申请的总内存（MB）
	NumCPU         int          `json:"num_cpu"`          // CPU 核心数
	GoVersion      string       `json:"go_version"`       // Go 版本
	HTTPMetrics    []HTTPMetric `json:"http_metrics"`     // HTTP 请求统计详情
	HTTPTotal      float64      `json:"http_total"`       // HTTP 总请求数
	HTTPErrors     float64      `json:"http_errors"`      // HTTP 5xx 错误数
	HTTPP99Seconds float64      `json:"http_p99_seconds"` // HTTP 请求 P99 延迟（秒）
}

// HTTPMetric 单个 HTTP 方法+路径的请求统计。
type HTTPMetric struct {
	Path   string  `json:"path"`   // 请求路径
	Method string  `json:"method"` // HTTP 方法
	Status string  `json:"status"` // 响应状态码
	Count  float64 `json:"count"`  // 累计请求次数
}

// Stats 返回系统监控数据。
// GET /api/v1/monitor/stats
//
// 数据来源：
//   - 运行时指标：runtime.MemStats + runtime 包
//   - HTTP 指标：Prometheus DefaultGatherer 采集的中间件指标
//   - 计算 HTTP 总请求数、5xx 错误数、P99 延迟
func (h *MonitorHandler) Stats(c *gin.Context) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	// 构建基础运行时指标
	stats := MonitorStats{
		UptimeSeconds: int64(time.Since(startTime).Seconds()),
		NumGoroutines: runtime.NumGoroutine(),
		MemoryAllocMB: float64(mem.Alloc) / 1024 / 1024,
		MemoryTotalMB: float64(mem.Sys) / 1024 / 1024,
		NumCPU:        runtime.NumCPU(),
		GoVersion:     runtime.Version(),
	}

	// 从 Prometheus 采集 HTTP 指标
	gatherer := monitorGatherer
	metricFamilies, err := gatherer.Gather()
	if err == nil {
		for _, mf := range metricFamilies {
			// 采集 HTTP 请求计数器
			if mf.GetName() == "http_requests_total" {
				for _, m := range mf.GetMetric() {
					labels := m.GetLabel()
					hm := HTTPMetric{}
					for _, l := range labels {
						switch l.GetName() {
						case "path":
							hm.Path = l.GetValue()
						case "method":
							hm.Method = l.GetValue()
						case "status":
							hm.Status = l.GetValue()
						}
					}
					hm.Count = m.GetCounter().GetValue()
					stats.HTTPTotal += hm.Count
					// 5xx 错误统计
					if len(hm.Status) > 0 && hm.Status[0] == '5' {
						stats.HTTPErrors += hm.Count
					}
					stats.HTTPMetrics = append(stats.HTTPMetrics, hm)
				}
			}

			// 从 Histogram 估算 P99 延迟
			if mf.GetName() == "http_request_duration_seconds" {
				for _, m := range mf.GetMetric() {
					hist := m.GetHistogram()
					if hist != nil && hist.GetSampleCount() > 0 {
						buckets := hist.GetBucket()
						totalCount := float64(hist.GetSampleCount())
						for _, b := range buckets {
							// 找到累计占比 >= 99% 的第一个桶
							if float64(b.GetCumulativeCount()) >= totalCount*0.99 {
								stats.HTTPP99Seconds = b.GetUpperBound()
								break
							}
						}
					}
				}
			}
		}
	}

	response.Success(c, stats)
}
