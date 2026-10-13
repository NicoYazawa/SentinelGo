package token

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func TestBlacklistRedisLifecycle(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis unavailable: %v", err)
	}
	oldSecret, oldRedis := JWTSecret, RDB
	Init(testSecret, rdb)
	jti := "coverage-" + uuid.NewString()
	t.Cleanup(func() { rdb.Del(ctx, "token:blacklist:"+jti); rdb.Close(); JWTSecret, RDB = oldSecret, oldRedis })
	if IsBlacklisted(jti) {
		t.Fatal("new token already blacklisted")
	}
	if err := BlacklistToken(jti, time.Minute); err != nil {
		t.Fatal(err)
	}
	if !IsBlacklisted(jti) {
		t.Fatal("blacklisted token accepted")
	}
	if ttl := rdb.TTL(ctx, "token:blacklist:"+jti).Val(); ttl <= 0 || ttl > time.Minute {
		t.Fatalf("incorrect blacklist lifetime: %v", ttl)
	}
	if err := rdb.Del(ctx, "token:blacklist:"+jti).Err(); err != nil {
		t.Fatal(err)
	}
	if IsBlacklisted(jti) {
		t.Fatal("removed blacklist entry still active")
	}
}
