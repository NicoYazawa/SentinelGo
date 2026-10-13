package middleware

import (
	"sentinelgo/pkg/i18n"

	"github.com/gin-gonic/gin"
)

// I18n 解析客户端语言偏好并写入 Gin 上下文，同时回写 Content-Language 响应头。
//
// 注册建议：放在 Recovery 之后、业务路由之前（即全局中间件链靠前位置），
// 以保证后续 Recovery / 业务 Handler 都能通过 i18n.FromContext 取到语言。
func I18n() gin.HandlerFunc {
	return func(c *gin.Context) {
		lang := i18n.Parse(
			c.GetHeader(i18n.HeaderAcceptLanguage),
			c.GetHeader(i18n.HeaderLang),
		)
		i18n.SetInContext(c, lang)
		c.Header("Content-Language", string(lang))
		c.Next()
	}
}
