package idgen

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/redis/go-redis/v9"
)

// GetAutoWorkerID 通过 Redis 自动为当前节点分配 WorkerID。
//
// 分配策略：
//  1. 以本机 IP 查询 Redis Hash（idgen:worker_registry:{serviceName}），检查是否已注册
//  2. 已注册：直接复用上次分配的 ID，支持节点重启后保持 WorkerID 不变
//  3. 未注册：通过 Redis INCR 原子自增计数器（idgen:worker_counter:{serviceName}），
//     分配新的 WorkerID，并对 1024 取模循环使用
//  4. 将 IP → WorkerID 映射回写到 Redis Hash，供下次重启查询
//
// 此方案无需手动配置 WorkerID，节点自动注册，支持动态扩缩容。
func GetAutoWorkerID(rdb *redis.Client, serviceName string) (int64, error) {
	if rdb == nil {
		return 0, nil // Redis 不可用时默认 WorkerID=0（单机模式）
	}

	ctx := context.Background()
	localIP := getLocalIP()

	key := "idgen:worker_registry:" + serviceName

	// 第一步：检查本机 IP 是否已有分配的 WorkerID（重启复用）
	if idStr, err := rdb.HGet(ctx, key, localIP).Result(); err == nil {
		if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
			return id, nil
		}
	}

	// 第二步：原子自增分配新的 WorkerID
	counterKey := "idgen:worker_counter:" + serviceName
	workerID, err := rdb.Incr(ctx, counterKey).Result()
	if err != nil {
		return 0, fmt.Errorf("分配 WorkerID 失败: %w", err)
	}

	// 取模映射到 0-1023 范围（用完自动循环）
	actualID := (workerID - 1) % 1024

	// 第三步：回写 IP → WorkerID 映射，供重启时查询复用
	_ = rdb.HSet(ctx, key, localIP, actualID).Err()

	return actualID, nil
}

// getLocalIP 获取本机第一个非回环的 IPv4 地址。
// 获取失败时返回 "127.0.0.1"。
func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "unknown"
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			return ipnet.IP.String()
		}
	}
	return "127.0.0.1"
}
