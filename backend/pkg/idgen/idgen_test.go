package idgen

import "testing"

func TestNewSnowflake(t *testing.T) {
	if _, err := NewSnowflake(-1); err != ErrWorkerIDOutOfRange {
		t.Error("expected range error for negative id")
	}
	if _, err := NewSnowflake(1024); err != ErrWorkerIDOutOfRange {
		t.Error("expected range error for id 1024")
	}
	sf, err := NewSnowflake(1)
	if err != nil {
		t.Fatal(err)
	}
	if sf == nil {
		t.Fatal("nil snowflake")
	}
}

func TestNextID(t *testing.T) {
	sf, _ := NewSnowflake(5)
	seen := make(map[int64]bool)
	for i := 0; i < 2000; i++ {
		id, err := sf.NextID()
		if err != nil {
			t.Fatalf("NextID error: %v", err)
		}
		if id <= 0 {
			t.Errorf("id should be positive: %d", id)
		}
		seen[id] = true
	}
	if len(seen) != 2000 {
		t.Errorf("expected unique ids, got %d unique", len(seen))
	}
}

func TestInitGlobal(t *testing.T) {
	if err := Init(2); err != nil {
		t.Fatal(err)
	}
	if SF == nil {
		t.Error("global SF not set")
	}
}

func TestGetAutoWorkerIDNilRedis(t *testing.T) {
	id, err := GetAutoWorkerID(nil, "svc")
	if err != nil || id != 0 {
		t.Errorf("nil redis should yield 0, got %d %v", id, err)
	}
}

func TestGetLocalIP(t *testing.T) {
	if getLocalIP() == "" {
		t.Error("local ip should not be empty")
	}
}
