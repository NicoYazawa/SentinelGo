// Package redis 提供 go-redis 客户端连接池的工厂初始化。
//
// 封装连接池参数配置与启动健康检查，采用 fail-fast 策略——
// 连接失败直接 panic，避免服务在无 Redis 可用状态下继续运行。
package redis

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// RedisConfig 定义 Redis 客户端连接池的配置参数。
type RedisConfig struct {
	Addr         string // Redis 服务地址，格式为 host:port
	Password     string // 认证密码，无密码传空字符串
	DB           int    // Redis 数据库编号（0-15），默认 0
	PoolSize     int    // 连接池最大 socket 连接数，建议设为 CPU 核数 * 2
	MinIdleConns int    // 连接池最小空闲连接数，避免冷启动延迟
}

// NewRedisClient 创建并验证 Redis 客户端连接。
//
// 初始化步骤：
//  1. 以配置参数创建 go-redis 客户端
//  2. 执行 PING 命令验证连接可达性
//  3. PING 失败直接 panic（fail-fast）
//
// 返回已就绪的 *redis.Client，可直接用于业务操作。
func NewRedisClient(cfg RedisConfig) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,     // 连接池最大连接数
		MinIdleConns: cfg.MinIdleConns, // 保持的最小空闲连接，减少冷启动延迟
	})

	// 启动时验证连接可达性
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		panic(fmt.Sprintf("Redis 连接失败: %v", err))
	}

	return rdb
}
