package queue

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// Producer 是 Redis Stream 任务生产者，负责任务投递。
type Producer struct {
	rdb *redis.Client
}

// NewProducer 创建生产者实例。
func NewProducer(rdb *redis.Client) *Producer {
	return &Producer{rdb: rdb}
}

// Publish 投递即时任务到 Redis Stream。
//
// 参数：
//   - ctx: 上下文（用于链路追踪和超时控制）
//   - task: 任务定义（Stream 名称 + Payload）
//
// 返回消息 ID（Redis 自动生成的时间序列 ID，如 "1680000000000-0"）。
// Stream 使用 Approx 近似截断，长度接近 MaxLen 时删除最早的消息。
func (p *Producer) Publish(ctx context.Context, task Task) (string, error) {
	if task.Payload == nil {
		task.Payload = make(map[string]any)
	}
	return p.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: task.Stream,
		MaxLen: DefaultMaxLen, // 近似限制 Stream 最大长度
		Approx: true,          // Approx 模式性能更好，允许少量超出
		Values: task.Payload,
	}).Result()
}

// PublishDelay 投递延时任务到 ZSet 延迟队列。
//
// 参数：
//   - stream: 到期后投递的目标 Stream 名称
//   - payload: 任务负载（内部会自动附加 __target_stream 元数据）
//   - executeAt: Unix 时间戳（秒），任务计划执行时间
//
// StartDelayScheduler 每秒扫描 ZSet，将到期任务原子移动到目标 Stream。
func (p *Producer) PublishDelay(ctx context.Context, stream string, payload map[string]any, executeAt int64) error {
	payload["__target_stream"] = stream // 标记目标 Stream，调度器据此投递
	payloadBytes := mapToBytes(payload)
	return p.rdb.ZAdd(ctx, "queue:delay", redis.Z{
		Score:  float64(executeAt),
		Member: string(payloadBytes),
	}).Err()
}
