package queue

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
)

func TestPendingCountReflectsStreamLength(t *testing.T) {
	rdb := testRedis(t)
	ctx := context.Background()
	stream := uniqueStream(t)
	t.Cleanup(func() { rdb.Del(ctx, stream) })
	consumer := NewConsumer(rdb, stream, "coverage-group")
	if err := rdb.Del(ctx, stream).Err(); err != nil {
		t.Fatal(err)
	}
	if consumer.PendingCount(ctx) != 0 {
		t.Fatal("missing stream must have no pending messages")
	}
	for i := 0; i < 2; i++ {
		if err := rdb.XAdd(ctx, &redis.XAddArgs{Stream: stream, Values: map[string]any{"payload": "data"}}).Err(); err != nil {
			t.Fatal(err)
		}
	}
	if got := consumer.PendingCount(ctx); got != 2 {
		t.Fatalf("pending count=%d want 2", got)
	}
}
