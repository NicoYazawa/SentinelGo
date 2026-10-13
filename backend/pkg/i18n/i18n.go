// Package i18n 提供最小可用的语言协商与上下文管理能力（当前支持 zh-CN / en-US）。
//
// 设计目标：
//   - 语言协商：X-Lang 显式覆盖 > Accept-Language 协商 > Default
//   - 通过 Gin 上下文在请求生命周期内传递解析出的语言
//   - 业务代码（response / validator 等）按上下文语言返回对应文案
//
// 后续扩展更多语言只需在 Supported / normalize / 各文案映射中追加即可。
package i18n

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// Lang 表示受支持的语言标签（BCP 47 风格）。
type Lang string

const (
	ZhCN Lang = "zh-CN"
	EnUS Lang = "en-US"

	// Default 是未显式指定时的回退语言。
	Default = ZhCN

	// contextKey 用于把解析出的语言存入 Gin 上下文。
	contextKey = "i18n_lang"

	// HeaderLang 允许客户端通过 X-Lang 强制覆盖语言（便于调试/测试/用户显式选择）。
	HeaderLang = "X-Lang"

	// HeaderAcceptLanguage 标准语言协商头。
	HeaderAcceptLanguage = "Accept-Language"
)

// Supported 返回所有受支持的语言标签。
func Supported() []Lang {
	return []Lang{ZhCN, EnUS}
}

// Parse 根据客户端偏好解析出受支持的语言。
// 优先级：X-Lang 显式覆盖 > Accept-Language 协商 > Default。
func Parse(acceptHeader, overrideHeader string) Lang {
	if overrideHeader != "" {
		if l := Normalize(overrideHeader); l != "" {
			return l
		}
	}
	if l := negotiate(acceptHeader); l != "" {
		return l
	}
	return Default
}

// negotiate 按 Accept-Language 中语言的出现顺序返回第一个受支持的语言。
func negotiate(header string) Lang {
	if header == "" {
		return ""
	}
	for _, part := range strings.Split(header, ",") {
		// 形如 "en-US;q=0.9" 或 "zh-CN"
		tag := strings.TrimSpace(part)
		if idx := strings.Index(tag, ";"); idx >= 0 {
			tag = tag[:idx]
		}
		if l := Normalize(strings.TrimSpace(tag)); l != "" {
			return l
		}
	}
	return ""
}

// Normalize 将任意语言标签归一化为受支持语言，无法匹配返回空。
// 例如 "zh-CN" / "zh" → ZhCN，"en" / "en-US" → EnUS。
func Normalize(tag string) Lang {
	t := strings.ToLower(strings.TrimSpace(tag))
	switch {
	case strings.HasPrefix(t, "zh"):
		return ZhCN
	case strings.HasPrefix(t, "en"):
		return EnUS
	}
	return ""
}

// SetInContext 将语言存入 Gin 上下文。
func SetInContext(c *gin.Context, lang Lang) {
	c.Set(contextKey, lang)
}

// FromContext 从 Gin 上下文取语言，缺失时返回 Default。
func FromContext(c *gin.Context) Lang {
	if v, ok := c.Get(contextKey); ok {
		if l, ok := v.(Lang); ok && l != "" {
			return l
		}
	}
	return Default
}
