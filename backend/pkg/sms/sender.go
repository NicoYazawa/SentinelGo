package sms

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	dypnsapi20170525 "github.com/alibabacloud-go/dypnsapi-20170525/v3/client"
	"github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/tea"
	credential "github.com/aliyun/credentials-go/credentials"
)

// Sender 是短信下发通道的抽象，便于在开发态、真实网关与测试桩之间切换。
type Sender interface {
	// Send 将验证码投递到指定手机号。
	Send(ctx context.Context, phone, code string) error
}

// DevSender 开发态发送器：仅将验证码打印到日志，不真正下发短信。
// 便于本地联调时从服务端日志直接获取验证码。
type DevSender struct{}

// NewDevSender 创建开发态发送器。
func NewDevSender() *DevSender { return &DevSender{} }

// Send 开发态：打印验证码到结构化日志。
func (d *DevSender) Send(_ context.Context, phone, code string) error {
	slog.Info("SMS 验证码（开发模式，未真实下发）",
		"phone", phone,
		"code", code,
	)
	return nil
}

// AliyunDypnsSender 基于阿里云「号码认证服务」(Dypnsapi) 下发验证码。
// 验证码由业务侧生成并作为参数传入模板，阿里云仅负责把短信投递到用户手机。
type AliyunDypnsSender struct {
	client       *dypnsapi20170525.Client
	signName     string
	templateCode string
	codeTTL      time.Duration
}

// NewAliyunDypnsSender 创建阿里云发送器。
// 凭证通过 credentials.NewCredential(nil) 走默认凭据链读取环境变量
// ALIBABA_CLOUD_ACCESS_KEY_ID / ALIBABA_CLOUD_ACCESS_KEY_SECRET，不落明文。
func NewAliyunDypnsSender(signName, templateCode string, codeTTL time.Duration) (*AliyunDypnsSender, error) {
	if codeTTL <= 0 {
		codeTTL = 5 * time.Minute
	}
	cred, err := credential.NewCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("阿里云凭据初始化失败: %w", err)
	}
	cfg := &openapi.Config{Credential: cred}
	cfg.Endpoint = tea.String("dypnsapi.aliyuncs.com")
	client, err := dypnsapi20170525.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("阿里云客户端创建失败: %w", err)
	}
	return &AliyunDypnsSender{
		client:       client,
		signName:     signName,
		templateCode: templateCode,
		codeTTL:      codeTTL,
	}, nil
}

// Send 调用 SendSmsVerifyCode 将验证码投递到用户手机。
func (a *AliyunDypnsSender) Send(_ context.Context, phone, code string) error {
	req := &dypnsapi20170525.SendSmsVerifyCodeRequest{
		SignName:     tea.String(a.signName),
		TemplateCode: tea.String(a.templateCode),
		TemplateParam: tea.String(fmt.Sprintf(
			`{"code":"%s","min":"%d"}`, code, int(a.codeTTL.Minutes()),
		)),
	}
	runtime := &service.RuntimeOptions{}
	if _, err := a.client.SendSmsVerifyCodeWithOptions(req, runtime); err != nil {
		return fmt.Errorf("阿里云发送短信失败: %w", err)
	}
	return nil
}
