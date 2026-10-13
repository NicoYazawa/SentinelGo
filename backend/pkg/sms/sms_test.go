package sms

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"sentinelgo/pkg/i18n"
)

// Hold concurrent reads at the boundary to reproduce the read/delete race.
type simultaneousReads struct {
	*memoryStore
	readers sync.WaitGroup
}

func (s *simultaneousReads) getCode(ctx context.Context, key string) (string, bool, error) {
	value, ok, err := s.memoryStore.getCode(ctx, key)
	s.readers.Done()
	s.readers.Wait()
	return value, ok, err
}

func TestCodeCanOnlyBeConsumedOnceConcurrently(t *testing.T) {
	const count = 16
	st := &simultaneousReads{memoryStore: &memoryStore{m: make(map[string]*memItem)}}
	st.readers.Add(count)
	st.setCode(context.Background(), codeKey(SceneBind, "13800000001"), "123456", time.Minute)
	svc := &SMSService{store: st, cfg: Config{CodeTTL: time.Minute, MaxAttempts: 5}}
	var successes atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if svc.VerifyCode(context.Background(), SceneBind, "13800000001", "123456") {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	if got := successes.Load(); got != 1 {
		t.Fatalf("same code accepted %d times; want 1", got)
	}
}

func TestAttemptLimitInvalidatesCode(t *testing.T) {
	svc := NewSMSService(nil, NewDevSender(), Config{MaxAttempts: 3})
	code, err := svc.SendCode(context.Background(), SceneLogin, "13800000001", i18n.ZhCN)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if svc.VerifyCode(context.Background(), SceneLogin, "13800000001", "wrong") {
			t.Fatal("wrong code accepted")
		}
	}
	if svc.VerifyCode(context.Background(), SceneLogin, "13800000001", code) {
		t.Fatal("code survived attempt limit")
	}
}
