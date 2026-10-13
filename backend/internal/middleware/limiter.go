package middleware

import (
	"net/http"
	"sync"

	"sentinelgo/pkg/response"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// IPRateLimiter 是基于 IP 的令牌桶限流器。
// 每个独立 IP 拥有自己的令牌桶，互不影响。
type IPRateLimiter struct {
	mu       sync.Mutex               // 互斥锁，保护 limiters map 的并发访问
	limiters map[string]*rate.Limiter // IP → 令牌桶映射
	perIP    rate.Limit               // 每个 IP 的令牌生成速率（token/s）
	burst    int                      // 每个 IP 的桶容量（允许的最大突发请求数）
}

// GlobalLimiter 是全局 IP 限流器实例。
// 默认配置：单 IP 每秒 50 个令牌，最大突发 100 个请求。
var GlobalLimiter = NewIPRateLimiter(50, 100)

// NewIPRateLimiter 创建 IP 限流器。
// r: 令牌生成速率（token/s），b: 桶容量（突发上限）。
func NewIPRateLimiter(r rate.Limit, b int) *IPRateLimiter {
	return &IPRateLimiter{
		limiters: make(map[string]*rate.Limiter),
		perIP:    r,
		burst:    b,
	}
}

// getLimiter 获取或创建指定 IP 的令牌桶（线程安全）。
func (l *IPRateLimiter) getLimiter(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()

	lim, exists := l.limiters[ip]
	if !exists {
		// 首次出现的 IP，创建新令牌桶
		lim = rate.NewLimiter(l.perIP, l.burst)
		l.limiters[ip] = lim
	}
	return lim
}

// RateLimit 是自适应令牌桶限流中间件，双层防护。
//
// 第一层：全局限流 — 整个服务每秒最多处理 5000 个请求（桶容量 10000）
//
//	超限时返回 HTTP 429，防止突发流量打垮服务。
//
// 第二层：单 IP 限流 — 每个客户端 IP 每秒最多 50 个请求（桶容量 100）
//
//	超限时返回 HTTP 429，防止单个用户滥用接口。
//
// 两层限流同时生效：必须同时通过全局和 IP 级别的令牌检查才能放行。
func RateLimit() gin.HandlerFunc {
	// 全局限流器（作用于所有请求，面向整个服务）
	globalLimiter := rate.NewLimiter(5000, 10000)

	return func(c *gin.Context) {
		// 第一层：全局限流检查
		if !globalLimiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, response.Response{
				Code:    40029,
				Msg:     "当前访问人数过多，服务器正在平滑限流降级，请稍后再试",
				TraceID: c.GetString(TraceIDKey),
				Data:    nil,
			})
			return
		}

		// 第二层：单 IP 限流检查
		ip := c.ClientIP()
		if !GlobalLimiter.getLimiter(ip).Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, response.Response{
				Code:    40029,
				Msg:     "请求过于频繁，请稍后再试",
				TraceID: c.GetString(TraceIDKey),
				Data:    nil,
			})
			return
		}

		c.Next()
	}
}
