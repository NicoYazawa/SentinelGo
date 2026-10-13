package utils

import (
	"fmt"

	"sentinelgo/pkg/i18n"

	"github.com/go-playground/validator/v10"
)

// validationMsg 按语言返回 校验标签 → 提示模板（使用 %s 占位参数）。
var validationMsg = map[i18n.Lang]map[string]string{
	i18n.ZhCN: {
		"required": "不能为空",
		"email":    "格式不正确",
		"min":      "长度不能小于 %s",
		"max":      "长度不能大于 %s",
		"len":      "长度必须为 %s",
		"eq":       "必须等于 %s",
		"gt":       "必须大于 %s",
		"gte":      "必须大于等于 %s",
		"lt":       "必须小于 %s",
		"lte":      "必须小于等于 %s",
		"oneof":    "必须是 [%s] 之一",
		"url":      "URL 格式不正确",
		"uuid":     "UUID 格式不正确",
	},
	i18n.EnUS: {
		"required": "is required",
		"email":    "is invalid",
		"min":      "must be at least %s characters",
		"max":      "must be at most %s characters",
		"len":      "must be exactly %s characters",
		"eq":       "must equal %s",
		"gt":       "must be greater than %s",
		"gte":      "must be greater than or equal to %s",
		"lt":       "must be less than %s",
		"lte":      "must be less than or equal to %s",
		"oneof":    "must be one of [%s]",
		"url":      "must be a valid URL",
		"uuid":     "must be a valid UUID",
	},
}

// fallbackMsg 未识别校验标签或语言时的兜底文案。
var fallbackMsg = map[i18n.Lang]string{
	i18n.ZhCN: "请求参数有误",
	i18n.EnUS: "Invalid request parameters",
}

// TranslateValidationError 将 validator 的英文校验错误翻译为指定语言的提示。
//
// 返回格式示例（中文）："{Field} 不能为空"；英文："{Field} is required"。
// 未识别的标签或语言返回兜底文案（fallbackMsg）。
func TranslateValidationError(err error, lang i18n.Lang) string {
	msgs, ok := validationMsg[lang]
	if !ok {
		msgs = validationMsg[i18n.Default]
	}
	if errs, ok := err.(validator.ValidationErrors); ok {
		for _, e := range errs {
			if tpl, ok := msgs[e.Tag()]; ok {
				return e.Field() + " " + fmt.Sprintf(tpl, e.Param())
			}
		}
	}
	if f, ok := fallbackMsg[lang]; ok {
		return f
	}
	return fallbackMsg[i18n.Default]
}
