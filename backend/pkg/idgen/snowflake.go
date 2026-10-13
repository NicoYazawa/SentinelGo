// Package idgen 提供分布式唯一 ID 生成能力，基于 Twitter Snowflake 算法。
//
// ID 结构（64 位）：
//
//		┌─┬──────────────────────────┬────────────┬──────────────┐
//		│0│    41-bit 时间戳(ms)     │10-bit 机器 │12-bit 序列号 │
//		└─┴──────────────────────────┴────────────┴──────────────┘
//
//	  - 1 位保留（始终为 0，保证 ID 为正数）
//	  - 41 位毫秒时间戳（从自定义 epoch 开始，可用约 69 年）
//	  - 10 位 WorkerID（支持 0-1023 共 1024 个节点，通过 Redis 自动分配）
//	  - 12 位序列号（单节点每毫秒最多生成 4096 个 ID）
//
// 时钟回拨处理：
//   - ≤5ms 回拨：Sleep 等待时钟追上
//   - >5ms 回拨：拒绝生成并返回 ErrClockBackwards
package idgen

import (
	"errors"
	"sync"
	"time"
)

// Snowflake 算法常量定义。
const (
	epoch          int64 = 1767225600000               // 自定义起始时间戳：2026-01-01 00:00:00 UTC（毫秒）
	workerIDBits   uint  = 10                          // WorkerID 占用位数
	maxWorkerID    int64 = -1 ^ (-1 << workerIDBits)   // 最大 WorkerID = 1023
	sequenceBits   uint  = 12                          // 序列号占用位数
	workerIDShift  uint  = sequenceBits                // WorkerID 左移位数 = 12
	timestampShift uint  = sequenceBits + workerIDBits // 时间戳左移位数 = 22
	sequenceMask   int64 = -1 ^ (-1 << sequenceBits)   // 序列号掩码 = 4095
)

var (
	// ErrWorkerIDOutOfRange WorkerID 不在 0-1023 范围内时返回
	ErrWorkerIDOutOfRange = errors.New("worker ID 超出合法范围 (0-1023)")
	// ErrClockBackwards 时钟回拨超过 5ms 时返回，拒绝生成 ID
	ErrClockBackwards = errors.New("时钟发生严重回拨，拒绝生成ID")
)

// Snowflake 是 Snowflake ID 生成器实例，所有方法并发安全。
type Snowflake struct {
	mu            sync.Mutex // 互斥锁，保证并发调用时 sequence 和 lastTimestamp 的一致性
	lastTimestamp int64      // 上次生成 ID 的时间戳（毫秒）
	workerID      int64      // 当前节点的 WorkerID（0-1023）
	sequence      int64      // 当前毫秒内的序列号（0-4095）
}

// SF 全局 Snowflake 实例，由 main.go 在启动时初始化。
var SF *Snowflake

// Init 初始化全局 Snowflake 实例。
func Init(workerID int64) error {
	sf, err := NewSnowflake(workerID)
	if err != nil {
		return err
	}
	SF = sf
	return nil
}

// NewSnowflake 创建 Snowflake 实例，校验 WorkerID 合法性。
func NewSnowflake(workerID int64) (*Snowflake, error) {
	if workerID < 0 || workerID > maxWorkerID {
		return nil, ErrWorkerIDOutOfRange
	}
	return &Snowflake{workerID: workerID}, nil
}

// NextID 生成下一个全局唯一 ID。
//
// 算法流程：
//  1. 获取当前毫秒时间戳
//  2. 检测时钟回拨：≤5ms 则等待，>5ms 则拒绝
//  3. 同一毫秒内序列号递增，超过 4095 则阻塞到下一毫秒
//  4. 跨毫秒时序列号重置为 0
//  5. 组装：时间戳(41位) | WorkerID(10位) | 序列号(12位)
func (s *Snowflake) NextID() (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()

	// 时钟回拨检测
	if now < s.lastTimestamp {
		offset := s.lastTimestamp - now
		if offset <= 5 {
			// 轻微回拨（≤5ms）：主动等待时钟追上
			time.Sleep(time.Duration(offset) * time.Millisecond)
			now = time.Now().UnixMilli()
			if now < s.lastTimestamp {
				return 0, ErrClockBackwards // 等待后仍未追上，拒绝生成
			}
		} else {
			// 严重回拨（>5ms）：直接拒绝，避免 ID 冲突
			return 0, ErrClockBackwards
		}
	}

	// 同一毫秒内处理
	if s.lastTimestamp == now {
		// 序列号递增 + 掩码截断（4095 后归零）
		s.sequence = (s.sequence + 1) & sequenceMask
		if s.sequence == 0 {
			// 当前毫秒序列号耗尽，阻塞直到下一毫秒
			for now <= s.lastTimestamp {
				now = time.Now().UnixMilli()
			}
		}
	} else {
		// 进入新的一毫秒，序列号从 0 开始
		s.sequence = 0
	}

	s.lastTimestamp = now

	// 位运算组装 64 位 ID
	id := ((now - epoch) << timestampShift) |
		(s.workerID << workerIDShift) |
		s.sequence

	return id, nil
}
