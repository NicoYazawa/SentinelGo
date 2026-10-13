package queue

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// testRedis 返回连上真实 Redis 的客户端；若环境无 Redis 则跳过测试。
func testRedis(t *testing.T) *redis.Client {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("redis 不可用，跳过: %v", err)
	}
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}

// uniqueStream 为每个测试生成独立的 Stream 名称，避免相互干扰。
func uniqueStream(t *testing.T) string {
	t.Helper()
	name := strings.ReplaceAll(t.Name(), "/", "_")
	return "ut:" + name + ":" + uuid.New().String()[:8]
}

func TestProducerPublish(t *testing.T) {
	rdb := testRedis(t)
	stream := uniqueStream(t)
	defer rdb.Del(context.Background(), stream)

	prod := NewProducer(rdb)
	id, err := prod.Publish(context.Background(), Task{
		Stream:  stream,
		Payload: map[string]any{"x": "1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if id == "" {
		t.Error("期望非空消息 ID")
	}
	info, err := rdb.XInfoStream(context.Background(), stream).Result()
	if err != nil {
		t.Fatal(err)
	}
	if info.Length != 1 {
		t.Errorf("期望 Stream 长度为 1，实际 %d", info.Length)
	}
}

func TestProducerPublishNilPayload(t *testing.T) {
	rdb := testRedis(t)
	stream := uniqueStream(t)
	defer rdb.Del(context.Background(), stream)

	prod := NewProducer(rdb)
	// 空 payload 在 Redis 下 XAdd 非法（需至少一个字段），应返回 error。
	if _, err := prod.Publish(context.Background(), Task{Stream: stream}); err == nil {
		t.Error("期望空 payload 返回 error")
	}
}

func TestProducerPublishDelay(t *testing.T) {
	rdb := testRedis(t)
	stream := uniqueStream(t)
	defer rdb.Del(context.Background(), stream, delayQueueKey)

	prod := NewProducer(rdb)
	payload := map[string]any{"job": "send"}
	if err := prod.PublishDelay(context.Background(), stream, payload, time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	// 延时任务应进入 ZSet 延迟队列
	n, err := rdb.ZCard(context.Background(), delayQueueKey).Result()
	if err != nil {
		t.Fatal(err)
	}
	if n < 1 {
		t.Errorf("期望延迟队列至少 1 条，实际 %d", n)
	}
}

func TestConsumerStartConsume(t *testing.T) {
	rdb := testRedis(t)
	stream := uniqueStream(t)
	defer rdb.Del(context.Background(), stream, stream+DLQSuffix)

	prod := NewProducer(rdb)
	cons := NewConsumer(rdb, stream, "ut_workers")
	defer cons.Stop()

	var called atomic.Bool
	var captured map[string]any
	cons.StartConsume(func(payload map[string]any) error {
		captured = payload
		// Publish the payload before the completion flag synchronizes the reader.
		called.Store(true)
		return nil
	})

	if _, err := prod.Publish(context.Background(), Task{
		Stream:  stream,
		Payload: map[string]any{"k": "v"},
	}); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(8 * time.Second)
	for !called.Load() && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if !called.Load() {
		t.Fatal("消费者未处理任何消息")
	}
	if captured["k"] != "v" {
		t.Errorf("handler 收到的 payload 不正确: %v", captured)
	}

	// 等待 ACK 生效，确认无残留 pending
	time.Sleep(300 * time.Millisecond)
	pending, err := rdb.XPending(context.Background(), stream, "ut_workers").Result()
	if err != nil {
		t.Fatal(err)
	}
	if pending.Count != 0 {
		t.Errorf("期望 pending 为 0，实际 %d", pending.Count)
	}
}

func TestConsumerHandleFailureRetry(t *testing.T) {
	rdb := testRedis(t)
	stream := uniqueStream(t)
	defer rdb.Del(context.Background(), stream, stream+DLQSuffix)

	cons := NewConsumer(rdb, stream, "ut_workers")
	msg := redis.XMessage{ID: "1-0", Values: map[string]any{"a": "b"}}
	cons.handleFailure(context.Background(), msg, errors.New("boom"))

	// 未超过最大重试次数：应重新入队（Stream 长度 +1）
	info, err := rdb.XInfoStream(context.Background(), stream).Result()
	if err != nil {
		t.Fatal(err)
	}
	if info.Length < 1 {
		t.Errorf("期望重试重新入队，长度 %d", info.Length)
	}
}

func TestConsumerHandleFailureDLQ(t *testing.T) {
	rdb := testRedis(t)
	stream := uniqueStream(t)
	defer rdb.Del(context.Background(), stream, stream+DLQSuffix)

	cons := NewConsumer(rdb, stream, "ut_workers")
	// 已达最大重试次数：应转入死信队列
	msg := redis.XMessage{ID: "1-0", Values: map[string]any{RetryCountField: DefaultMaxRetry}}
	cons.handleFailure(context.Background(), msg, errors.New("boom"))

	dlq := stream + DLQSuffix
	info, err := rdb.XInfoStream(context.Background(), dlq).Result()
	if err != nil {
		t.Fatal(err)
	}
	if info.Length < 1 {
		t.Errorf("期望死信队列至少 1 条，实际 %d", info.Length)
	}
}

func TestGetRetryCount(t *testing.T) {
	if got := getRetryCount(nil); got != 0 {
		t.Errorf("nil map 期望 0，实际 %d", got)
	}
	if got := getRetryCount(map[string]any{}); got != 0 {
		t.Errorf("空 map 期望 0，实际 %d", got)
	}
	if got := getRetryCount(map[string]any{RetryCountField: 2}); got != 2 {
		t.Errorf("int 期望 2，实际 %d", got)
	}
	if got := getRetryCount(map[string]any{RetryCountField: "5"}); got != 5 {
		t.Errorf("string 期望 5，实际 %d", got)
	}
}

func TestPublishDelayAndScheduler(t *testing.T) {
	rdb := testRedis(t)
	stream := uniqueStream(t)
	defer rdb.Del(context.Background(), stream, delayQueueKey)

	prod := NewProducer(rdb)
	payload := map[string]any{"job": "delayed"}
	if err := prod.PublishDelay(context.Background(), stream, payload, time.Now().Unix()); err != nil {
		t.Fatal(err)
	}

	StartDelayScheduler(rdb)

	deadline := time.Now().Add(5 * time.Second)
	for {
		info, err := rdb.XInfoStream(context.Background(), stream).Result()
		if err == nil && info.Length >= 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("延时任务未被调度器投递到目标 Stream")
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestReportMetrics(t *testing.T) {
	rdb := testRedis(t)
	stream := uniqueStream(t)
	defer rdb.Del(context.Background(), stream)

	// 至少触发一次指标采集 goroutine 启动，不阻塞等待（内部 15s 轮询）
	ReportMetrics(rdb, []string{stream})
	// 验证指标对象已注册可用
	if QueuePendingMessages == nil {
		t.Error("QueuePendingMessages 未初始化")
	}
}
