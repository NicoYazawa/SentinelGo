// Package queue 提供基于 Redis Stream 的异步任务队列能力。
//
// 核心特性：
//   - 生产者：Publish 投递即时任务，PublishDelay 投递延时任务
//   - 消费者：XReadGroup 阻塞消费，支持消费者组负载均衡
//   - 死信队列：任务重试 3 次仍失败后自动转入 DLQ（{stream}:dead_letter）
//   - 延时队列：基于 Redis ZSet 实现，每秒扫描到期任务并投递
//   - 指标采集：每 15 秒上报各 Stream 积压消息数到 Prometheus
//
// Redis 数据结构：
//   - Stream: 任务队列本体（如 "queue:order"）
//   - Consumer Group: 消费者组（如 "order_workers"）
//   - ZSet key "queue:delay": 延时任务，score 为执行时间戳
//
// 任务重试流程：
//  1. 消费者处理失败 → 重试计数 +1
//  2. 重新 XAdd 回同一 Stream（队尾）
//  3. ACK 原消息
//  4. 重试超过 3 次 → 转入死信队列，人工介入
package queue

import (
	"encoding/json"
	"strconv"
)

// 队列常量定义。
const (
	DefaultMaxLen   = 10000          // Stream 默认最大长度（超出时删除最早消息）
	DefaultMaxRetry = 3              // 默认最大重试次数
	DLQSuffix       = ":dead_letter" // 死信队列后缀，如 "queue:order:dead_letter"
	TraceIDField    = "_trace_id"    // 消息体中 TraceID 字段名
	RetryCountField = "_retry_count" // 消息体中重试计数字段名
)

// Task 是投递到队列的异步任务定义。
type Task struct {
	Stream  string         // 目标 Stream 名称
	Payload map[string]any // 任务负载数据
}

// TaskHandler 是消费者处理任务的回调函数类型。
// 返回 nil 表示处理成功（自动 ACK），返回 error 表示处理失败（触发重试或转入 DLQ）。
type TaskHandler func(payload map[string]any) error

// mapToBytes 将 Payload 序列化为 JSON 字节数组。
func mapToBytes(m map[string]any) []byte {
	b, _ := json.Marshal(m)
	return b
}

// itoa 将 int64 转为字符串，用于 ZSet 范围查询参数。
func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}
