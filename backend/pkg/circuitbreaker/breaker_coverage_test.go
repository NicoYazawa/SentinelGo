package circuitbreaker

import (
	"errors"
	"testing"

	"github.com/sony/gobreaker"
)

func TestDefaultReadyToTrip(t *testing.T) {
	if defaultReadyToTrip(gobreaker.Counts{Requests: 9, ConsecutiveFailures: 6}) {
		t.Error("请求数不足 10 不应熔断")
	}
	if defaultReadyToTrip(gobreaker.Counts{Requests: 10, ConsecutiveFailures: 5}) {
		t.Error("连续失败未超过 5 不应熔断")
	}
	if !defaultReadyToTrip(gobreaker.Counts{Requests: 10, ConsecutiveFailures: 6}) {
		t.Error("满足请求数且连续失败>5 应熔断")
	}
}

func TestExternalReadyToTrip(t *testing.T) {
	if externalReadyToTrip(gobreaker.Counts{Requests: 4, TotalFailures: 4}) {
		t.Error("请求数不足 5 不应熔断")
	}
	if !externalReadyToTrip(gobreaker.Counts{Requests: 5, TotalFailures: 3}) {
		t.Error("失败率≥50% 应熔断")
	}
}

func TestLogStateChange(t *testing.T) {
	// 内部为 slog.Warn，确保不 panic
	logStateChange("test", gobreaker.StateClosed, gobreaker.StateOpen)
}

func TestBreakerTripsAndFallback(t *testing.T) {
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name: "test",
		ReadyToTrip: func(c gobreaker.Counts) bool {
			return c.Requests >= 1
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			logStateChange(name, from, to)
		},
	})
	// 第一次失败即熔断，应返回 fallback
	v, err := Execute(cb, func() (int, error) { return 0, errors.New("boom") }, 99)
	if err == nil || v != 99 {
		t.Errorf("熔断后应返回 fallback: %d %v", v, err)
	}
	// 熔断打开后不应再执行原函数，直接返回 fallback
	called := false
	v, _ = Execute(cb, func() (int, error) { called = true; return 1, nil }, 7)
	if called {
		t.Error("打开状态不应执行原函数")
	}
	if v != 7 {
		t.Errorf("打开状态 fallback 应为 7，got %d", v)
	}
}
