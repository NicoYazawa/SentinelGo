package redis

import "testing"

// TestNewRedisClientUnreachable verifies fail-fast behavior: a client that
// cannot reach Redis must panic during initialization.
func TestNewRedisClientUnreachable(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on unreachable Redis")
		}
	}()
	NewRedisClient(RedisConfig{Addr: "127.0.0.1:1"})
}
