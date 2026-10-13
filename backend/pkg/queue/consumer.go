package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Consumer 是 Redis Stream 消费者组，支持多实例负载均衡消费。
// 同一消费组内的消费者自动分担消息，每条消息只会被组内一个消费者处理。
type Consumer struct {
	rdb          *redis.Client // Redis 客户端
	stream       string        // 监听的 Stream 名称
	group        string        // 消费者组名称
	consumerName string        // 当前消费者实例的唯一名称
	done         chan struct{} // 关闭信号，用于优雅停止消费循环
	stopOnce     sync.Once
}

// NewConsumer 创建消费者，自动初始化消费者组（不存在时创建）。
//
// 参数：
//   - rdb: Redis 客户端
//   - stream: Stream 名称
//   - group: 消费者组名称（多个消费者实例共用同一 group 实现负载均衡）
//
// XGroupCreateMkStream：若 Stream 或 Group 不存在则自动创建，从当前最新消息开始消费。
func NewConsumer(rdb *redis.Client, stream, group string) *Consumer {
	ctx := context.Background()
	// MKSTREAM 模式：Stream 不存在时自动创建，消费者组不存在时自动创建
	_ = rdb.XGroupCreateMkStream(ctx, stream, group, "0").Err()

	return &Consumer{
		rdb:          rdb,
		stream:       stream,
		group:        group,
		consumerName: fmt.Sprintf("%s-%s", group, uuid.New().String()[:8]),
	}
}

// StartConsume 启动阻塞消费循环（后台 goroutine）。
//
// 消费逻辑：
//  1. XReadGroup 阻塞读取（5 秒超时），每次拉取最多 10 条
//  2. 逐条调用 handler 处理
//  3. 处理成功 → XAck 确认
//  4. 处理失败 → handleFailure 重试/死信
//
// ">" 表示只读取从未分配给任何消费者的新消息。
func (c *Consumer) StartConsume(handler TaskHandler) {
	c.done = make(chan struct{})
	go func() {
		ctx := context.Background()
		for {
			// 优雅退出：收到关闭信号后结束消费循环
			select {
			case <-c.done:
				return
			default:
			}

			// 阻塞读取：5 秒无消息则返回空，避免空轮询
			streams, err := c.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    c.group,
				Consumer: c.consumerName,
				Streams:  []string{c.stream, ">"}, // ">" 意为只读新消息
				Count:    10,                      // 每次最多拉取 10 条
				Block:    5 * time.Second,         // 阻塞等待时间
			}).Result()

			if err != nil && err != redis.Nil {
				slog.Error("MQ 拉取消息失败", "error", err)
				time.Sleep(1 * time.Second) // 出错时短暂休眠，避免疯狂重试
				continue
			}

			for _, stream := range streams {
				for _, msg := range stream.Messages {
					if err := handler(msg.Values); err == nil {
						// 处理成功：确认消息
						c.rdb.XAck(ctx, c.stream, c.group, msg.ID)
					} else {
						// 处理失败：进入重试或死信流程
						c.handleFailure(ctx, msg, err)
					}
				}
			}
		}
	}()
}

// Stop 优雅停止消费循环。可安全重复调用。
func (c *Consumer) Stop() {
	if c.done == nil {
		return
	}
	c.stopOnce.Do(func() {
		close(c.done)
	})
}

// handleFailure 处理消费失败的消息：重试计数递增 → 超过阈值转入 DLQ → 否则重新入队。
func (c *Consumer) handleFailure(ctx context.Context, msg redis.XMessage, bizErr error) {
	retryCount := getRetryCount(msg.Values)
	retryCount++
	msg.Values[RetryCountField] = retryCount

	// 超过最大重试次数：转入死信队列（DLQ）
	if retryCount > DefaultMaxRetry {
		slog.Error("任务达到最大重试次数，转入死信队列",
			"msg_id", msg.ID,
			"stream", c.stream,
			"retry_count", retryCount,
			"error", bizErr,
		)
		dlqStream := c.stream + DLQSuffix
		// 将失败消息写入死信队列（保留原始 Payload，含重试计数和 TraceID）
		_ = c.rdb.XAdd(ctx, &redis.XAddArgs{
			Stream: dlqStream,
			Values: msg.Values,
		}).Err()
		c.rdb.XAck(ctx, c.stream, c.group, msg.ID)
		return
	}

	// 未超阈值：重新投递到 Stream 队尾等待重试
	slog.Warn("任务处理失败，等待重试",
		"msg_id", msg.ID,
		"retry_count", retryCount,
		"error", bizErr,
	)
	_ = c.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: c.stream,
		Values: msg.Values,
	}).Err()
	c.rdb.XAck(ctx, c.stream, c.group, msg.ID)
}

// PendingCount 返回 Stream 中尚未被确认的消息总数（含待处理 + 未分发）。
func (c *Consumer) PendingCount(ctx context.Context) int64 {
	info, err := c.rdb.XInfoStream(ctx, c.stream).Result()
	if err != nil {
		return 0
	}
	return info.Length
}

// getRetryCount 从消息 Values 中提取当前重试次数。
// Redis Stream Values 统一为 map[string]any，数字可能被反序列化为 string 或 int。
func getRetryCount(values map[string]any) int {
	if v, ok := values[RetryCountField]; ok {
		switch val := v.(type) {
		case int:
			return val
		case string:
			var n int
			json.Unmarshal([]byte(val), &n)
			return n
		}
	}
	return 0
}
