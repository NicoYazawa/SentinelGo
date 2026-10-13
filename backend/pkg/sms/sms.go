// Package sms 提供短信验证码的生成、存储与校验能力。
//
// 设计要点（生产化版本）：
//   - 验证码由业务侧生成随机 6 位数字，存储到 Redis（带 TTL），由外部 Sender 投递。
//   - 存储抽象为 store 接口：Redis 可用时用 redisStore，否则降级为进程内 memoryStore，
//     保证本地无 Redis 时服务仍可启动（与 token 包的 Redis 可选策略一致）。
//   - 按「场景」(scene) 隔离验证码：同一手机号在登录/绑定/重置场景下互不覆盖。
//   - 安全加固：发送频率限制（ratelimit）、验证码一次性使用、连续错误次数超限即作废。
package sms

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"sentinelgo/pkg/i18n"
	"sentinelgo/pkg/queue"
)

// 场景常量：同一手机号在不同业务下验证码相互隔离。
const (
	SceneLogin = "login" // 短信登录 / 无感注册
	SceneBind  = "bind"  // 绑定手机号
	SceneReset = "reset" // 找回密码 / 重置密码
)

// SMSStream 是短信异步下发任务使用的 Redis Stream 名称。
// 与 main.go 中启动的消费者组 (sms_workers) 一一对应。
const SMSStream = "stream:sms"

// Config 短信服务运行参数（由 config.SMSCfg 映射而来）。
type Config struct {
	CodeTTL     time.Duration // 验证码有效期
	RateLimit   time.Duration // 同一手机号发送频率限制
	MaxAttempts int           // 单码最大校验尝试次数（防爆破）
}

// SMSService 管理短信验证码的生成、存储、限频与校验。
type SMSService struct {
	sender   Sender
	store    store
	cfg      Config
	producer *queue.Producer // 可选：非空时发送改为异步投递到队列
}

// NewSMSService 创建短信服务实例。
// rdb 为 nil 时自动降级为进程内存储；各项配置为零值时采用安全默认值。
func NewSMSService(rdb *redis.Client, sender Sender, cfg Config) *SMSService {
	var st store
	if rdb != nil {
		st = &redisStore{rdb: rdb}
	} else {
		st = &memoryStore{m: make(map[string]*memItem)}
	}
	if cfg.CodeTTL <= 0 {
		cfg.CodeTTL = 5 * time.Minute
	}
	if cfg.RateLimit <= 0 {
		cfg.RateLimit = 60 * time.Second
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 5
	}
	return &SMSService{sender: sender, store: st, cfg: cfg}
}

// phoneRe 校验中国大陆手机号格式。
var phoneRe = regexp.MustCompile(`^1[3-9]\d{9}$`)

// smsMsg 是短信服务面向用户的提示文案（按语言）。
var smsMsg = map[i18n.Lang]map[string]string{
	i18n.ZhCN: {
		"phoneInvalid": "手机号格式不正确",
		"rateLimit":    "验证码发送过于频繁，请 %d 秒后再试",
	},
	i18n.EnUS: {
		"phoneInvalid": "Invalid phone number",
		"rateLimit":    "Please try again in %d seconds",
	},
}

func smsMessage(lang i18n.Lang, key string, args ...any) string {
	tpl, ok := smsMsg[lang][key]
	if !ok {
		tpl = smsMsg[i18n.Default][key]
	}
	return fmt.Sprintf(tpl, args...)
}

// normalizePhone 校验手机号格式（仅做格式校验）。
func normalizePhone(phone string) bool {
	return phoneRe.MatchString(phone)
}

// genCode 生成 6 位随机数字验证码（使用 crypto/rand，避免可预测）。
func genCode() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(900000))
	return fmt.Sprintf("%06d", n.Int64()+100000)
}

func codeKey(scene, phone string) string    { return fmt.Sprintf("sms:code:%s:%s", scene, phone) }
func attemptKey(scene, phone string) string { return fmt.Sprintf("sms:attempt:%s:%s", scene, phone) }
func rateKey(scene, phone string) string    { return fmt.Sprintf("sms:ratelimit:%s:%s", scene, phone) }

// SendCode 生成验证码、执行发送限频、落库存储并下发。
// 返回生成的验证码（供开发模式回传前端，生产模式调用方应忽略）。
// 任何一步失败都会回滚已写入的验证码，避免用户拿到无效码。
func (s *SMSService) SendCode(ctx context.Context, scene, phone string, lang i18n.Lang) (string, error) {
	if !normalizePhone(phone) {
		return "", errors.New(smsMessage(lang, "phoneInvalid"))
	}

	// 频率限制：同一场景+手机号在 RateLimit 窗口内只能发一次
	ok, err := s.store.setNX(ctx, rateKey(scene, phone), s.cfg.RateLimit)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", errors.New(smsMessage(lang, "rateLimit", int(s.cfg.RateLimit.Seconds())))
	}

	code := genCode()
	if err := s.store.setCode(ctx, codeKey(scene, phone), code, s.cfg.CodeTTL); err != nil {
		_ = s.store.del(ctx, rateKey(scene, phone))
		return "", err
	}

	if s.producer != nil {
		// 异步模式：将发送任务投递到 Redis Stream，由独立消费者协程调用真实 Sender 下发，
		// 解耦接口响应与第三方短信网关延迟。投递失败视为下发失败，回滚验证码与限频标记。
		if _, err := s.producer.Publish(ctx, queue.Task{
			Stream:  SMSStream,
			Payload: map[string]any{"phone": phone, "code": code},
		}); err != nil {
			_ = s.store.del(ctx, codeKey(scene, phone))
			_ = s.store.del(ctx, rateKey(scene, phone))
			return "", err
		}
		return code, nil
	}

	if err := s.sender.Send(ctx, phone, code); err != nil {
		// 下发失败：撤回验证码，允许用户稍后重试（不受限频阻断太久）
		_ = s.store.del(ctx, codeKey(scene, phone))
		_ = s.store.del(ctx, rateKey(scene, phone))
		return "", err
	}
	return code, nil
}

// SetProducer 启用异步下发模式。
// 传入非 nil 的 Producer 后，SendCode 将发送任务投递到队列而非同步调用 Sender；
// 传入 nil 恢复为同步下发（适用于本地无 Redis 或单元测试场景）。
func (s *SMSService) SetProducer(p *queue.Producer) {
	s.producer = p
}

// VerifyCode 校验验证码。过期、不匹配或错误次数超限均返回 false（不抛 error，避免泄露细节）。
func (s *SMSService) VerifyCode(ctx context.Context, scene, phone, code string) bool {
	if !normalizePhone(phone) || code == "" {
		return false
	}
	ok, err := s.store.consume(ctx, codeKey(scene, phone), attemptKey(scene, phone), code, s.cfg.MaxAttempts)
	return err == nil && ok
}

// ---- 存储抽象 ----

// store 屏蔽 Redis 与进程内两种后端，对上层提供统一语义。
type store interface {
	consume(ctx context.Context, codeKey, attemptKey, code string, maxAttempts int) (bool, error)
	setNX(ctx context.Context, key string, ttl time.Duration) (bool, error)        // 限频：不存在则写入并返回 true
	setCode(ctx context.Context, key, code string, ttl time.Duration) error        // 写入验证码
	getCode(ctx context.Context, key string) (string, bool, error)                 // 读取验证码（false=不存在/已过期）
	del(ctx context.Context, key string) error                                     // 删除
	incrAttempt(ctx context.Context, key string, ttl time.Duration) (int64, error) // 尝试次数+1
}

// redisStore 基于 go-redis 的存储实现。
type redisStore struct{ rdb *redis.Client }

var consumeCodeScript = redis.NewScript(`
local stored = redis.call('GET', KEYS[1])
if not stored then return 0 end
if stored == ARGV[1] then
 redis.call('DEL', KEYS[1], KEYS[2])
 return 1
end
local attempts = redis.call('INCR', KEYS[2])
if attempts == 1 then redis.call('PEXPIRE', KEYS[2], math.max(redis.call('PTTL', KEYS[1]), 1)) end
if attempts >= tonumber(ARGV[2]) then redis.call('DEL', KEYS[1], KEYS[2]) end
return 0
`)

func (r *redisStore) consume(ctx context.Context, key, attempts, code string, maxAttempts int) (bool, error) {
	n, err := consumeCodeScript.Run(ctx, r.rdb, []string{key, attempts}, code, maxAttempts).Int()
	return n == 1, err
}

func (r *redisStore) setNX(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return r.rdb.SetNX(ctx, key, "1", ttl).Result()
}

func (r *redisStore) setCode(ctx context.Context, key, code string, ttl time.Duration) error {
	return r.rdb.Set(ctx, key, code, ttl).Err()
}

func (r *redisStore) getCode(ctx context.Context, key string) (string, bool, error) {
	v, err := r.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}

func (r *redisStore) del(ctx context.Context, key string) error {
	return r.rdb.Del(ctx, key).Err()
}

func (r *redisStore) incrAttempt(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	n, err := r.rdb.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if n == 1 {
		_ = r.rdb.Expire(ctx, key, ttl).Err()
	}
	return n, nil
}

// memItem 是进程内存储的单元。
type memItem struct {
	val string
	exp time.Time
	cnt int
}

// memoryStore 进程内降级存储，并发安全。仅在 Redis 不可用时使用。
type memoryStore struct {
	mu sync.Mutex
	m  map[string]*memItem
}

func (m *memoryStore) getLocked(key string) *memItem {
	it, ok := m.m[key]
	if !ok {
		return nil
	}
	if time.Now().After(it.exp) {
		delete(m.m, key)
		return nil
	}
	return it
}

func (m *memoryStore) consume(_ context.Context, key, attemptKey, code string, maxAttempts int) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	item := m.getLocked(key)
	if item == nil {
		delete(m.m, attemptKey)
		return false, nil
	}
	if item.val == code {
		delete(m.m, key)
		delete(m.m, attemptKey)
		return true, nil
	}
	attempt := m.getLocked(attemptKey)
	if attempt == nil {
		attempt = &memItem{exp: item.exp}
		m.m[attemptKey] = attempt
	}
	attempt.cnt++
	if attempt.cnt >= maxAttempts {
		delete(m.m, key)
		delete(m.m, attemptKey)
	}
	return false, nil
}

func (m *memoryStore) setNX(_ context.Context, key string, ttl time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.getLocked(key) != nil {
		return false, nil
	}
	m.m[key] = &memItem{val: "1", exp: time.Now().Add(ttl)}
	return true, nil
}

func (m *memoryStore) setCode(_ context.Context, key, code string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.m[key] = &memItem{val: code, exp: time.Now().Add(ttl)}
	return nil
}

func (m *memoryStore) getCode(_ context.Context, key string) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	it := m.getLocked(key)
	if it == nil {
		return "", false, nil
	}
	return it.val, true, nil
}

func (m *memoryStore) del(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.m, key)
	return nil
}

func (m *memoryStore) incrAttempt(_ context.Context, key string, ttl time.Duration) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	it := m.getLocked(key)
	if it == nil {
		it = &memItem{exp: time.Now().Add(ttl)}
		m.m[key] = it
	}
	it.cnt++
	return int64(it.cnt), nil
}
