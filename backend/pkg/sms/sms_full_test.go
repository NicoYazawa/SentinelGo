package sms

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"sentinelgo/pkg/i18n"
	"sentinelgo/pkg/queue"
)

// testRedis 返回连上真实 Redis 的客户端；若环境无 Redis 则跳过。
func testRedis(t *testing.T) *redis.Client {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("redis 不可用，跳过: %v", err)
	}
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}

func TestNormalizePhone(t *testing.T) {
	if !normalizePhone("13800000000") {
		t.Error("合法手机号被拒绝")
	}
	if normalizePhone("12345") {
		t.Error("非法短号被接受")
	}
	if normalizePhone("23800000000") {
		t.Error("非法号段被接受")
	}
	if normalizePhone("1380000000a") {
		t.Error("含字母被接受")
	}
}

func TestGenCode(t *testing.T) {
	re := regexp.MustCompile(`^\d{6}$`)
	for i := 0; i < 200; i++ {
		if c := genCode(); !re.MatchString(c) {
			t.Fatalf("验证码格式错误: %q", c)
		}
	}
}

func TestSmsMessage(t *testing.T) {
	if got := smsMessage(i18n.ZhCN, "phoneInvalid"); got == "" {
		t.Error("中文文案不应为空")
	}
	// 未知语言应回退到默认文案，而非空串
	if got := smsMessage(i18n.Lang("xx"), "phoneInvalid"); got == "" {
		t.Error("未知语言应回退默认文案")
	}
}

func TestSendCodeInvalidPhone(t *testing.T) {
	svc := NewSMSService(nil, NewDevSender(), Config{})
	if _, err := svc.SendCode(context.Background(), SceneLogin, "123", i18n.ZhCN); err == nil {
		t.Error("非法手机号应返回错误")
	}
}

func TestVerifyCodeInvalid(t *testing.T) {
	svc := NewSMSService(nil, NewDevSender(), Config{})
	if svc.VerifyCode(context.Background(), SceneLogin, "bad", "123456") {
		t.Error("非法手机号应校验失败")
	}
	if svc.VerifyCode(context.Background(), SceneLogin, "13800000000", "") {
		t.Error("空验证码应校验失败")
	}
}

func TestRedisStoreBasics(t *testing.T) {
	rdb := testRedis(t)
	ctx := context.Background()
	st := &redisStore{rdb: rdb}
	key := "ut:sms:basic:" + time.Now().Format("150405.000")
	defer st.del(ctx, key)

	if err := st.setCode(ctx, key, "111111", time.Minute); err != nil {
		t.Fatal(err)
	}
	v, ok, err := st.getCode(ctx, key)
	if err != nil || !ok || v != "111111" {
		t.Errorf("getCode 异常: %q %v %v", v, ok, err)
	}

	if n, err := st.incrAttempt(ctx, key+"_a", time.Minute); err != nil || n != 1 {
		t.Errorf("incrAttempt 异常: %d %v", n, err)
	}

	k2 := key + "_nx"
	defer st.del(ctx, k2)
	if ok, _ := st.setNX(ctx, k2, time.Minute); !ok {
		t.Error("首次 setNX 应为 true")
	}
	if ok, _ := st.setNX(ctx, k2, time.Minute); ok {
		t.Error("重复 setNX 应为 false")
	}
}

func TestRedisStoreConsumeSuccess(t *testing.T) {
	rdb := testRedis(t)
	ctx := context.Background()
	st := &redisStore{rdb: rdb}
	key := "ut:sms:consume"
	att := "ut:sms:consume_att"
	defer st.del(ctx, key)
	defer st.del(ctx, att)

	st.setCode(ctx, key, "222222", time.Minute)
	if ok, _ := st.consume(ctx, key, att, "222222", 5); !ok {
		t.Error("正确验证码应消费成功")
	}
	if _, ok, _ := st.getCode(ctx, key); ok {
		t.Error("消费后验证码应被删除")
	}
}

func TestRedisStoreConsumeAttemptLimit(t *testing.T) {
	rdb := testRedis(t)
	ctx := context.Background()
	st := &redisStore{rdb: rdb}
	key := "ut:sms:attlimit"
	att := "ut:sms:attlimit_att"
	defer st.del(ctx, key)
	defer st.del(ctx, att)

	st.setCode(ctx, key, "333333", time.Minute)
	for i := 0; i < 5; i++ {
		if ok, _ := st.consume(ctx, key, att, "wrong", 5); ok {
			t.Fatalf("第 %d 次错误验证码不应消费成功", i+1)
		}
	}
	// 达到最大尝试次数后，正确验证码也应失效
	if ok, _ := st.consume(ctx, key, att, "333333", 5); ok {
		t.Error("超过最大尝试次数后验证码应失效")
	}
}

func TestSendCodeSyncWithRedis(t *testing.T) {
	rdb := testRedis(t)
	ctx := context.Background()
	svc := NewSMSService(rdb, NewDevSender(), Config{})
	phone := "13800001111"
	code, err := svc.SendCode(ctx, SceneLogin, phone, i18n.ZhCN)
	if err != nil {
		t.Fatal(err)
	}
	if code == "" {
		t.Error("验证码不应为空")
	}
	if !svc.VerifyCode(ctx, SceneLogin, phone, code) {
		t.Error("正确验证码应校验通过")
	}
	if svc.VerifyCode(ctx, SceneLogin, phone, code) {
		t.Error("验证码消费后应失效")
	}
}

func TestSendCodeRateLimitRedis(t *testing.T) {
	rdb := testRedis(t)
	ctx := context.Background()
	svc := NewSMSService(rdb, NewDevSender(), Config{RateLimit: time.Minute})
	phone := "13800002222"
	if _, err := svc.SendCode(ctx, SceneLogin, phone, i18n.ZhCN); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SendCode(ctx, SceneLogin, phone, i18n.ZhCN); err == nil {
		t.Error("频率限制内重复发送应返回错误")
	}
}

func TestSendCodeAsync(t *testing.T) {
	rdb := testRedis(t)
	ctx := context.Background()
	prod := queue.NewProducer(rdb)
	svc := NewSMSService(rdb, NewDevSender(), Config{})
	svc.SetProducer(prod)

	before, _ := rdb.XLen(ctx, SMSStream).Result()
	phone := "13800003333"
	code, err := svc.SendCode(ctx, SceneLogin, phone, i18n.ZhCN)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := rdb.XLen(ctx, SMSStream).Result()
	if after <= before {
		t.Errorf("异步模式下消息应进入 %s", SMSStream)
	}
	// 异步分支同样写入了验证码，校验应成功
	if !svc.VerifyCode(ctx, SceneLogin, phone, code) {
		t.Error("异步路径也应落库验证码")
	}
}

func TestMemoryStoreGetAndIncr(t *testing.T) {
	st := &memoryStore{m: make(map[string]*memItem)}
	ctx := context.Background()
	st.setCode(ctx, "k", "1", time.Minute)
	v, ok, _ := st.getCode(ctx, "k")
	if !ok || v != "1" {
		t.Errorf("memoryStore.getCode 异常: %q %v", v, ok)
	}
	if n, _ := st.incrAttempt(ctx, "a", time.Minute); n != 1 {
		t.Errorf("memoryStore.incrAttempt 异常: %d", n)
	}
	// setNX 二次返回 false
	st.setNX(ctx, "nx", time.Minute)
	if ok, _ := st.setNX(ctx, "nx", time.Minute); ok {
		t.Error("memoryStore 二次 setNX 应为 false")
	}
}

func TestNewAliyunDypnsSender(t *testing.T) {
	// Dummy environment credentials select a deterministic provider; no request is sent.
	t.Setenv("ALIBABA_CLOUD_ACCESS_KEY_ID", "test-access-key")
	t.Setenv("ALIBABA_CLOUD_ACCESS_KEY_SECRET", "test-access-secret")
	for _, ttl := range []time.Duration{time.Minute, 0, -time.Minute} {
		t.Run(ttl.String(), func(t *testing.T) {
			sender, err := NewAliyunDypnsSender("sign", "tpl", ttl)
			if err != nil {
				t.Fatal(err)
			}
			if sender == nil || sender.client == nil {
				t.Fatal("发送器或客户端为空")
			}
			wantTTL := ttl
			if wantTTL <= 0 {
				wantTTL = 5 * time.Minute
			}
			if sender.signName != "sign" || sender.templateCode != "tpl" || sender.codeTTL != wantTTL {
				t.Fatalf("发送器配置错误: %+v", sender)
			}
		})
	}
}
