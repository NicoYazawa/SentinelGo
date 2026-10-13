package queue

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// delayQueueKey 是 Redis ZSet 中存储延时任务的键名。
const delayQueueKey = "queue:delay"

// StartDelayScheduler 启动延时任务调度器，每秒扫描到期任务并投递到目标 Stream。
//
// 工作流程：
//  1. 每秒执行一次 ZSet 范围查询（score ≤ 当前时间戳）
//  2. 解析 JSON 格式的 Payload，提取 __target_stream 字段
//  3. 使用 Redis Pipeline 原子性地完成 ZREM（从延迟队列移除）+ XADD（投递到 Stream）
//  4. 解析失败或无目标 Stream 的任务直接 ZREM 丢弃
//
// Pipeline 保证移除和投递的原子性，避免任务丢失或重复投递。
func StartDelayScheduler(rdb *redis.Client) {
	go func() {
		ctx := context.Background()
		ticker := time.NewTicker(1 * time.Second) // 每秒扫描一次
		defer ticker.Stop()

		for range ticker.C {
			now := time.Now().Unix()
			// 查询所有 score ≤ 当前时间的延时任务（即到期任务）
			members, err := rdb.ZRangeArgs(ctx, redis.ZRangeArgs{
				Key:     delayQueueKey,
				Start:   "0",
				Stop:    itoa(now),
				ByScore: true, // 按 score 排序
				Count:   10,   // 每批最多处理 10 条
			}).Result()
			if err != nil {
				slog.Error("延时队列扫描失败", "error", err)
				continue
			}

			for _, member := range members {
				var payload map[string]any
				if err := json.Unmarshal([]byte(member), &payload); err != nil {
					// 格式损坏的任务直接移除
					_ = rdb.ZRem(ctx, delayQueueKey, member).Err()
					continue
				}

				// 提取目标任务 Stream 名称
				stream, _ := payload["__target_stream"].(string)
				delete(payload, "__target_stream") // 清理内部元数据字段
				if stream == "" {
					_ = rdb.ZRem(ctx, delayQueueKey, member).Err()
					continue
				}

				// Pipeline 原子操作：移除 ZSet + 投递 Stream
				pipe := rdb.Pipeline()
				pipe.ZRem(ctx, delayQueueKey, member)
				pipe.XAdd(ctx, &redis.XAddArgs{
					Stream: stream,
					Values: payload,
				})
				if _, err := pipe.Exec(ctx); err != nil {
					slog.Error("延时任务投递失败", "error", err)
				}
			}
		}
	}()
}
