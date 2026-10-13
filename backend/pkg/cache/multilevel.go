// Package cache 提供多级缓存能力，集成缓存三大防御模式：防穿透、防击穿、防雪崩。
//
// 架构设计：
//
//	L1: BigCache（本地内存，纳秒级读取）
//	L2: Redis（分布式缓存，毫秒级读取）
//	L3: Singleflight + 数据库回源（防击穿合并请求）
//
// 集群一致性：通过 Redis Pub/Sub 广播缓存失效通知，各节点同步清除本地 L1 缓存。
package cache

import (
	"context"
	"errors"
	"log/slog"
	"math/rand"
	"time"

	"github.com/allegro/bigcache/v3"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

// ErrNotFound 用于 dbFallback 返回"记录不存在"信号。
// 多级缓存检测到此错误会自动缓存空值（__EMPTY__），防止缓存穿透。
var ErrNotFound = errors.New("cache: record not found")

// MultiLevelCache 二级缓存实例，组合本地内存缓存与分布式 Redis 缓存。
//
// singleflight.Group 确保在高并发回源时，同一 key 只有一个请求穿透到数据库，
// 其他请求共享结果，有效防止缓存击穿。
type MultiLevelCache struct {
	local        *bigcache.BigCache // L1: 本地内存缓存（无网络开销）
	redis        *redis.Client      // L2: 分布式 Redis 缓存
	requestGroup singleflight.Group // 防击穿：合并同一 key 的并发回源请求
}

// MC 全局缓存实例，由 main.go 在启动时通过 Init 初始化。
var MC *MultiLevelCache

// Init 初始化全局多级缓存。
// rdb 为 nil 时仅使用本地缓存（适用于开发环境或 Redis 不可用的降级场景）。
func Init(rdb *redis.Client) {
	MC = newCache(rdb)
}

// newCache 创建多级缓存实例，启动集群缓存同步监听。
func newCache(rdb *redis.Client) *MultiLevelCache {
	// BigCache 配置：10 分钟 TTL，5 分钟清理间隔
	config := bigcache.DefaultConfig(10 * time.Minute)
	config.CleanWindow = 5 * time.Minute
	lc, err := bigcache.New(context.Background(), config)
	if err != nil {
		slog.Warn("BigCache 初始化失败，降级为纯 Redis 模式", "error", err)
	}

	mc := &MultiLevelCache{
		local: lc,
		redis: rdb,
	}

	// 启动后台协程，监听集群缓存清除通知
	go mc.subscribePurge()
	return mc
}

// GetOrLoad 多级缓存读取流程：L1 → L2 → Singleflight → DB。
//
// 参数：
//   - key: 缓存键
//   - ttl: 缓存过期时间（基础值，实际会叠加随机扰动防雪崩）
//   - dbFallback: 数据库回源函数，仅在缓存全部未命中时调用
//
// 防御机制：
//   - 防穿透：dbFallback 返回 ErrNotFound 时，缓存空值 __EMPTY__ 30 秒
//   - 防击穿：singleflight.Group.Do 合并同一 key 的并发回源请求
//   - 防雪崩：TTL 叠加 rand(0-300) 秒随机扰动，避免集中过期
func (m *MultiLevelCache) GetOrLoad(ctx context.Context, key string, ttl time.Duration, dbFallback func() (string, error)) (string, error) {
	// L1: 本地内存缓存（纳秒级，优先命中）
	if m.local != nil {
		if data, err := m.local.Get(key); err == nil {
			return string(data), nil
		}
	}

	// L2: Redis 分布式缓存（毫秒级，命中时回填 L1）
	if m.redis != nil {
		dataStr, err := m.redis.Get(ctx, key).Result()
		if err == nil {
			if m.local != nil {
				_ = m.local.Set(key, []byte(dataStr)) // 回填本地缓存
			}
			return dataStr, nil
		}
	}

	// L3: Singleflight 防击穿 + 回源数据库
	v, err, _ := m.requestGroup.Do(key, func() (any, error) {
		// 拿到 singleflight 锁后，再次检查 Redis（可能已被其他协程回填）
		if m.redis != nil {
			if dataStr, err := m.redis.Get(ctx, key).Result(); err == nil {
				return dataStr, nil
			}
		}

		// 回源数据库
		dbData, err := dbFallback()
		if err != nil {
			// 防穿透：对"记录不存在"缓存空值，避免恶意请求穿透到数据库
			if errors.Is(err, ErrNotFound) {
				if m.redis != nil {
					_ = m.redis.Set(ctx, key, "__EMPTY__", 30*time.Second).Err()
				}
				return "__EMPTY__", nil
			}
			return "", err
		}

		// 防雪崩：TTL 叠加随机扰动（0-300秒），避免大量缓存同时过期
		jitter := time.Duration(rand.Intn(300)) * time.Second
		actualTTL := ttl + jitter

		// 回填 L2 + L1
		if m.redis != nil {
			_ = m.redis.Set(ctx, key, dbData, actualTTL).Err()
		}
		if m.local != nil {
			_ = m.local.Set(key, []byte(dbData))
		}

		return dbData, nil
	})

	if err != nil {
		return "", err
	}
	return v.(string), nil
}

// Delete 缓存删除（Cache Aside 模式），同时通过 Pub/Sub 通知集群清除本地缓存。
// 先删本地 L1，再删 Redis L2，最后广播删除通知。
func (m *MultiLevelCache) Delete(ctx context.Context, key string) {
	if m.local != nil {
		_ = m.local.Delete(key)
	}
	if m.redis != nil {
		_ = m.redis.Del(ctx, key).Err()
		// 广播清除通知，集群其他节点收到后会清除本地 L1 缓存
		_ = m.redis.Publish(ctx, "cache:purge", key).Err()
	}
}

// subscribePurge 后台监听 Redis Pub/Sub 的 cache:purge 频道。
// 当集群其他节点删除了缓存，本节点收到通知后同步清除本地 L1 缓存，
// 保证集群内各节点的本地缓存最终一致。
func (m *MultiLevelCache) subscribePurge() {
	if m.redis == nil {
		return
	}
	sub := m.redis.Subscribe(context.Background(), "cache:purge")
	defer sub.Close()
	ch := sub.Channel()
	for msg := range ch {
		if m.local != nil {
			_ = m.local.Delete(msg.Payload)
		}
	}
}
