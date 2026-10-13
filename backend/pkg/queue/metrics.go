package queue

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
)

var (
	// QueuePendingMessages 是各 Stream 当前积压消息数的 Prometheus Gauge。
	// 标签 stream 区分不同的任务队列。
	QueuePendingMessages = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mq_pending_messages_count",
			Help: "Current number of pending messages in the stream.",
		},
		[]string{"stream"},
	)
)

// init 在包加载时自动注册 Prometheus 指标。
func init() {
	prometheus.MustRegister(QueuePendingMessages)
}

// ReportMetrics 启动后台指标采集 goroutine，每 15 秒检查各 Stream 积压量。
//
// 参数：
//   - rdb: Redis 客户端
//   - streams: 需要监控的 Stream 名称列表
//
// 采集方式：调用 XInfoStream 获取 Stream 的 length 字段（含未消费和待确认消息）。
func ReportMetrics(rdb *redis.Client, streams []string) {
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			for _, stream := range streams {
				info, err := rdb.XInfoStream(context.Background(), stream).Result()
				if err != nil {
					continue // 采集失败时跳过，等下一轮
				}
				QueuePendingMessages.WithLabelValues(stream).Set(float64(info.Length))
			}
		}
	}()
}
