package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

// Prometheus 指标定义。
//
// 指标命名遵循 Prometheus 最佳实践：
//   - Counter: http_requests_total（累计请求数，带 method/path/status 标签）
//   - Histogram: http_request_duration_seconds（请求耗时分布，10 个桶）
//
// 注意：Go/Process 采集器由 promhttp.Handler() 自动注册，
// 此处不可重复注册，否则会触发 init() panic。
var (
	// HttpRequestsTotal 统计每个 (method, path, status) 组合的请求总数。
	// 用于计算 QPS、错误率、Apdex 等指标。
	HttpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"method", "path", "status"},
	)

	// HttpRequestDuration 记录请求处理耗时的分布。
	// Buckets 设计覆盖从 5ms 到 5s 的范围，适用于绝大多数 API 场景。
	HttpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latencies in seconds.",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0},
		},
		[]string{"method", "path"},
	)
)

// init 在包加载时向 Prometheus DefaultRegistry 注册指标。
// promhttp.Handler() 会自动注册 Go/Process 采集器，此处无需重复。
func init() {
	prometheus.MustRegister(HttpRequestsTotal)
	prometheus.MustRegister(HttpRequestDuration)
}

// Metrics 是 Prometheus 指标采集中间件。
// 记录每个请求的耗时和计数，暴露给 /metrics 端点供 Prometheus 抓取。
func Metrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		// 先执行业务逻辑
		c.Next()

		// 记录请求耗时（秒）和计数
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())

		HttpRequestDuration.WithLabelValues(method, path).Observe(duration)
		HttpRequestsTotal.WithLabelValues(method, path, status).Inc()
	}
}
