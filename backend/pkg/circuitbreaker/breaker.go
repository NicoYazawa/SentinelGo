// Package circuitbreaker 提供熔断器封装，保护服务免受级联故障的影响。
//
// 熔断器三种状态：
//
//	Closed（闭合）  → 正常执行请求，统计失败率
//	Open（断开）    → 直接拒绝请求，快速失败，返回降级值
//	Half-Open（半开）→ 放行少量探测请求，成功则闭合，失败则重新断开
//
// 使用 sony/gobreaker 库实现，提供两种预配置的熔断器：
//   - DefaultBreaker：通用服务间调用保护
//   - ExternalAPIBreaker：外部 API 调用保护（更敏感的失败率阈值）
package circuitbreaker

import (
	"log/slog"
	"time"

	"github.com/sony/gobreaker"
)

var (
	// DefaultBreaker 通用熔断器，适用于内部服务间调用保护。
	// 参数说明：
	//   - MaxRequests=3：半开状态最多放行 3 个探测请求
	//   - Interval=60s：以 60 秒为统计窗口
	//   - Timeout=30s：熔断打开 30 秒后进入半开状态
	//   - ReadyToTrip：至少 10 个请求且连续失败 5 次以上才熔断
	DefaultBreaker = gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:          "default",
		MaxRequests:   3,
		Interval:      60 * time.Second,
		Timeout:       30 * time.Second,
		ReadyToTrip:   defaultReadyToTrip,
		OnStateChange: logStateChange,
	})

	// ExternalAPIBreaker 外部 API 调用熔断器，对失败更敏感。
	// 参数说明：
	//   - 至少 5 个请求且失败率 ≥50% 即熔断
	//   - 熔断后 60 秒进入半开探测
	ExternalAPIBreaker = gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:          "external_api",
		MaxRequests:   5,
		Interval:      60 * time.Second,
		Timeout:       60 * time.Second,
		ReadyToTrip:   externalReadyToTrip,
		OnStateChange: logStateChange,
	})
)

// defaultReadyToTrip 通用熔断触发条件：至少 10 个请求且连续失败 5 次以上。
func defaultReadyToTrip(counts gobreaker.Counts) bool {
	return counts.Requests >= 10 && counts.ConsecutiveFailures > 5
}

// externalReadyToTrip 外部 API 熔断触发条件：至少 5 个请求且失败率 ≥50%。
func externalReadyToTrip(counts gobreaker.Counts) bool {
	failRate := float64(counts.TotalFailures) / float64(counts.Requests)
	return counts.Requests >= 5 && failRate >= 0.5
}

// logStateChange 记录熔断器状态变更。
func logStateChange(name string, from, to gobreaker.State) {
	slog.Warn("熔断器状态变更",
		"name", name,
		"from", from.String(),
		"to", to.String(),
	)
}

// Execute 在熔断器保护下执行函数，熔断时返回 fallback 降级值。
// 使用 Go 泛型避免类型断言，调用方无需手动转换返回值类型。
//
// 使用示例：
//
//	result, err := circuitbreaker.Execute(DefaultBreaker, func() (string, error) {
//	    return callRemoteService()
//	}, "default_value")
func Execute[T any](cb *gobreaker.CircuitBreaker, fn func() (T, error), fallback T) (T, error) {
	result, err := cb.Execute(func() (any, error) {
		return fn()
	})
	if err != nil {
		return fallback, err
	}
	return result.(T), nil
}
