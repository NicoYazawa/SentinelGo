package idgen

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func TestNextIDClockBackwards(t *testing.T) {
	sf, _ := NewSnowflake(1)
	// 将上次时间戳设到未来 100s，触发严重时钟回拨拒绝
	sf.lastTimestamp = time.Now().UnixMilli() + 100000
	if _, err := sf.NextID(); err != ErrClockBackwards {
		t.Errorf("严重时钟回拨应返回 ErrClockBackwards, got %v", err)
	}
}

func TestNextIDSequenceRollover(t *testing.T) {
	sf, _ := NewSnowflake(1)
	// 极短时间内连续生成超过单毫秒容量(4096)的 ID，触发序列号翻转与跨毫秒阻塞
	var last int64
	for i := 0; i < 4097; i++ {
		id, err := sf.NextID()
		if err != nil {
			t.Fatal(err)
		}
		last = id
	}
	if last <= 0 {
		t.Error("ID 应为正数")
	}
}

func TestGetAutoWorkerIDWithRedis(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("redis 不可用，跳过: %v", err)
	}
	defer rdb.Close()
	ctx := context.Background()

	service := "ut_idgen_" + uuid.New().String()[:8]
	defer rdb.Del(ctx, "idgen:worker_registry:"+service, "idgen:worker_counter:"+service)

	id, err := GetAutoWorkerID(rdb, service)
	if err != nil {
		t.Fatal(err)
	}
	if id < 0 || id > 1023 {
		t.Errorf("worker id 超出范围 [0,1023]: %d", id)
	}
	// 同进程同 IP 再次调用应复用同一 ID（重启复用逻辑）
	id2, err := GetAutoWorkerID(rdb, service)
	if err != nil {
		t.Fatal(err)
	}
	if id2 != id {
		t.Errorf("重启复用应返回相同 ID: %d vs %d", id2, id)
	}
}
