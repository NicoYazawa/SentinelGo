package cache

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func redisClient(t *testing.T) *redis.Client {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("redis 不可用，跳过: %v", err)
	}
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}

func TestGetOrLoadRedisL2Hit(t *testing.T) {
	rdb := redisClient(t)
	ctx := context.Background()
	key := "ut:cache:l2hit:" + time.Now().String()
	defer rdb.Del(ctx, key)
	if err := rdb.Set(ctx, key, "v", time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	mc := newCache(rdb)
	called := false
	v, err := mc.GetOrLoad(ctx, key, time.Minute, func() (string, error) {
		called = true
		return "shouldnot", nil
	})
	if err != nil || v != "v" {
		t.Fatalf("L2 命中应返回 v, got %q %v", v, err)
	}
	if called {
		t.Error("L2 命中不应回源")
	}
}

func TestGetOrLoadRedisLoadAndStore(t *testing.T) {
	rdb := redisClient(t)
	ctx := context.Background()
	key := "ut:cache:load:" + time.Now().String()
	defer rdb.Del(ctx, key)
	mc := newCache(rdb)
	v, err := mc.GetOrLoad(ctx, key, time.Minute, func() (string, error) {
		return "dbv", nil
	})
	if err != nil || v != "dbv" {
		t.Fatalf("回源失败: %q %v", v, err)
	}
	// 再次读取应命中 L2，不再回源
	v2, err := mc.GetOrLoad(ctx, key, time.Minute, func() (string, error) {
		t.Error("不应二次回源")
		return "", nil
	})
	if err != nil || v2 != "dbv" {
		t.Fatalf("应从 L2 命中: %q %v", v2, err)
	}
	if got, _ := rdb.Get(ctx, key).Result(); got != "dbv" {
		t.Errorf("L2 应已写入 dbv, got %q", got)
	}
}

func TestGetOrLoadRedisNotFound(t *testing.T) {
	rdb := redisClient(t)
	ctx := context.Background()
	key := "ut:cache:notfound:" + time.Now().String()
	defer rdb.Del(ctx, key)
	mc := newCache(rdb)
	v, err := mc.GetOrLoad(ctx, key, time.Minute, func() (string, error) {
		return "", ErrNotFound
	})
	if err != nil || v != "__EMPTY__" {
		t.Fatalf("不存在应缓存空值: %q %v", v, err)
	}
	if got, _ := rdb.Get(ctx, key).Result(); got != "__EMPTY__" {
		t.Errorf("L2 应写入 __EMPTY__, got %q", got)
	}
}

func TestDeleteRedis(t *testing.T) {
	rdb := redisClient(t)
	ctx := context.Background()
	key := "ut:cache:del:" + time.Now().String()
	defer rdb.Del(ctx, key)
	if err := rdb.Set(ctx, key, "x", time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	mc := newCache(rdb)
	mc.Delete(ctx, key)
	if got, _ := rdb.Get(ctx, key).Result(); got != "" {
		t.Errorf("Delete 应清除 L2, got %q", got)
	}
}
