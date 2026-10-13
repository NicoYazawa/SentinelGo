package sms

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"sentinelgo/pkg/i18n"
	"sentinelgo/pkg/queue"
)

type errorSender struct{ err error }

func (s errorSender) Send(context.Context, string, string) error { return s.err }

type failingStore struct {
	*memoryStore
	nxError, codeError error
}

func (s *failingStore) setNX(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	if s.nxError != nil {
		return false, s.nxError
	}
	return s.memoryStore.setNX(ctx, key, ttl)
}
func (s *failingStore) setCode(ctx context.Context, key, code string, ttl time.Duration) error {
	if s.codeError != nil {
		return s.codeError
	}
	return s.memoryStore.setCode(ctx, key, code, ttl)
}

func TestSendFailureRollsBackCodeAndRateLimit(t *testing.T) {
	ctx := context.Background()
	errDelivery := errors.New("delivery unavailable")
	svc := NewSMSService(nil, errorSender{errDelivery}, Config{})
	code, err := svc.SendCode(ctx, SceneLogin, "13800004444", i18n.ZhCN)
	if !errors.Is(err, errDelivery) || code != "" {
		t.Fatalf("want delivery error and no code: %q %v", code, err)
	}
	if _, ok, err := svc.store.getCode(ctx, codeKey(SceneLogin, "13800004444")); err != nil || ok {
		t.Fatal("failed delivery left a usable code")
	}
	svc.sender = NewDevSender()
	code, err = svc.SendCode(ctx, SceneLogin, "13800004444", i18n.ZhCN)
	if err != nil || !svc.VerifyCode(ctx, SceneLogin, "13800004444", code) {
		t.Fatalf("failed delivery prevented retry: %v", err)
	}
}

func TestStoreFailureDoesNotLeaveThrottle(t *testing.T) {
	for _, stage := range []string{"rate", "code"} {
		t.Run(stage, func(t *testing.T) {
			ctx := context.Background()
			failure := errors.New("store failed")
			st := &failingStore{memoryStore: &memoryStore{m: make(map[string]*memItem)}}
			if stage == "rate" {
				st.nxError = failure
			} else {
				st.codeError = failure
			}
			svc := NewSMSService(nil, NewDevSender(), Config{})
			svc.store = st
			if code, err := svc.SendCode(ctx, SceneLogin, "13800005555", i18n.ZhCN); code != "" || !errors.Is(err, failure) {
				t.Fatalf("store error not propagated: %q %v", code, err)
			}
			if len(st.m) != 0 {
				t.Fatalf("store failure left state: %+v", st.m)
			}
		})
	}
}

func TestQueuePublishFailureRollsBackCode(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", MaxRetries: -1, DialTimeout: 100 * time.Millisecond})
	t.Cleanup(func() { rdb.Close() })
	svc := NewSMSService(nil, NewDevSender(), Config{})
	svc.SetProducer(queue.NewProducer(rdb))
	ctx := context.Background()
	if code, err := svc.SendCode(ctx, SceneLogin, "13800006666", i18n.ZhCN); err == nil || code != "" {
		t.Fatalf("failed publication should reject code: %q %v", code, err)
	}
	if _, ok, _ := svc.store.getCode(ctx, codeKey(SceneLogin, "13800006666")); ok {
		t.Fatal("publication failure left code")
	}
	svc.SetProducer(nil)
	if code, err := svc.SendCode(ctx, SceneLogin, "13800006666", i18n.ZhCN); err != nil || !svc.VerifyCode(ctx, SceneLogin, "13800006666", code) {
		t.Fatalf("publication failure prevented synchronous retry: %v", err)
	}
}

func TestMemoryExpiryAndDeletion(t *testing.T) {
	ctx := context.Background()
	st := &memoryStore{m: map[string]*memItem{"expired": {val: "123456", exp: time.Now().Add(-time.Second)}}}
	if _, ok, _ := st.getCode(ctx, "expired"); ok {
		t.Fatal("expired code accepted")
	}
	if _, present := st.m["expired"]; present {
		t.Fatal("expired entry not removed")
	}
	if _, ok, _ := st.getCode(ctx, "missing"); ok {
		t.Fatal("missing code accepted")
	}
	if err := st.setCode(ctx, "code", "123456", time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := st.del(ctx, "code"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := st.getCode(ctx, "code"); ok {
		t.Fatal("deleted code accepted")
	}
}
