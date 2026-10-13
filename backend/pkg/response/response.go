// Package response 提供统一的 HTTP JSON 响应格式与错误码体系。
//
// 设计原则：
//   - 所有响应 HTTP Status 均为 200，业务状态通过 code 字段区分
//   - 每个响应自动携带 trace_id，实现全链路错误追踪
//   - 参数校验失败时自动将 validator 英文错误翻译为中文提示
package response

import (
	"net/http"

	"sentinelgo/pkg/i18n"
	"sentinelgo/pkg/utils"

	"github.com/gin-gonic/gin"
)

// Response 是 API 统一响应结构体。
// code=0 表示成功，非 0 表示业务错误（见 code.go 中的常量定义）。
type Response struct {
	Code    int    `json:"code"`     // 业务状态码，0=成功，非0=错误
	Msg     string `json:"msg"`      // 提示信息
	TraceID string `json:"trace_id"` // 请求链路追踪 ID，用于日志关联与问题排查
	Data    any    `json:"data"`     // 响应数据负载
}

// Success 返回成功响应（code=0）。
func Success(c *gin.Context, data any) {
	traceID, _ := c.Get("trace_id")
	traceStr, _ := traceID.(string)

	c.JSON(http.StatusOK, Response{
		Code:    CodeSuccess,
		Msg:     "success",
		TraceID: traceStr,
		Data:    data,
	})
}

// Fail 返回业务错误响应。
// errCode 为业务错误码（见 code.go），customMsg 为空时自动从错误码映射表获取默认消息。
func Fail(c *gin.Context, errCode int, customMsg string) {
	traceID, _ := c.Get("trace_id")
	traceStr, _ := traceID.(string)

	msg := customMsg
	if msg == "" {
		// 按请求上下文语言从映射表获取本地化错误提示
		msg = GetErrorMsg(errCode, i18n.FromContext(c))
	}

	c.JSON(http.StatusOK, Response{
		Code:    errCode,
		Msg:     msg,
		TraceID: traceStr,
		Data:    gin.H{},
	})
}

// ShouldBindJSON 绑定 JSON 请求体并自动将 validator 校验错误翻译为中文提示。
//
// 返回值：true 表示绑定和校验成功，false 表示失败（已自动写入错误响应）。
// 所有 Handler 中使用此方法统一处理请求体绑定，避免重复的错误处理代码。
func ShouldBindJSON(c *gin.Context, obj any) bool {
	if err := c.ShouldBindJSON(obj); err != nil {
		// 将 GORM/validator 的英文错误信息翻译为用户可读的本地化提示
		Fail(c, ErrParamInvalid, utils.TranslateValidationError(err, i18n.FromContext(c)))
		return false
	}
	return true
}
