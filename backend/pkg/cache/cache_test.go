package cache

import (
	"context"
	"errors"
	"testing"
)

func TestGetOrLoadLocalOnly(t *testing.T) {
	Init(nil)
	ctx := context.Background()
	val, err := MC.GetOrLoad(ctx, "k1", 0, func() (string, error) {
		return "v1", nil
	})
	if err != nil || val != "v1" {
		t.Fatalf("got %q %v", val, err)
	}
	// second call should hit L1 without calling dbFallback
	val2, err := MC.GetOrLoad(ctx, "k1", 0, func() (string, error) {
		t.Fatal("dbFallback should not be called on cache hit")
		return "", nil
	})
	if err != nil || val2 != "v1" {
		t.Errorf("L1 hit wrong: %q %v", val2, err)
	}
}

func TestGetOrLoadNotFound(t *testing.T) {
	Init(nil)
	val, err := MC.GetOrLoad(context.Background(), "k2", 0, func() (string, error) {
		return "", ErrNotFound
	})
	if err != nil || val != "__EMPTY__" {
		t.Errorf("expected __EMPTY__, got %q %v", val, err)
	}
}

func TestGetOrLoadError(t *testing.T) {
	Init(nil)
	_, err := MC.GetOrLoad(context.Background(), "k3", 0, func() (string, error) {
		return "", errors.New("db down")
	})
	if err == nil {
		t.Error("expected error propagation")
	}
}

func TestDeleteLocalOnly(t *testing.T) {
	Init(nil)
	MC.Delete(context.Background(), "missing-key")
}

func TestSubscribePurgeNilRedis(t *testing.T) {
	mc := &MultiLevelCache{} // redis nil -> returns immediately
	mc.subscribePurge()
}
