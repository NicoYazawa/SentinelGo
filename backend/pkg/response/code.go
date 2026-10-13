package response

import "sentinelgo/pkg/i18n"

// 统一业务错误码常量定义。
//
// 编码规则（参考 HTTP 状态码分段思想）：
//
//	0          — 成功
//	1xxxx      — 系统级错误（服务器内部、参数校验）
//	2xxxx      — 认证授权错误（未登录、Token 过期、权限不足）
//	3xxxx      — 用户相关业务错误
//	4xxxx      — 资源/限流相关业务错误
const (
	CodeSuccess               = 0     // 请求成功
	ErrSystemError            = 10000 // 服务器内部错误（未知异常兜底）
	ErrParamInvalid           = 11000 // 请求参数格式不正确或校验失败
	ErrUnauthorized           = 20000 // 未登录或登录状态无效
	ErrTokenExpired           = 20001 // Access Token 已过期
	ErrForbidden              = 20002 // 当前用户无此操作的权限
	ErrPasswordChangeRequired = 20003 // 临时密码仅可用于设置正式密码
	ErrUserAlreadyExists      = 30001 // 手机号或邮箱已被注册
	ErrUserNotFound           = 30002 // 用户不存在
	ErrMenuNotAvailable       = 40001 // 菜单资源不可用（已禁用或不存在）
	ErrRateLimited            = 40029 // 请求过于频繁，被限流拦截
)

// codeMsgMap 维护 错误码 → 各语言提示信息 的映射表。
// 新增错误码时需在所有语言下同步添加映射，否则回退到 Default 语言或兜底文案。
var codeMsgMap = map[i18n.Lang]map[int]string{
	i18n.ZhCN: {
		CodeSuccess:               "success",
		ErrSystemError:            "服务器开小差了，请稍后再试",
		ErrParamInvalid:           "提交的参数格式不正确",
		ErrUnauthorized:           "未经授权的访问，请先登录",
		ErrTokenExpired:           "登录凭证已过期，请重新登录",
		ErrForbidden:              "权限不足，拒绝访问",
		ErrPasswordChangeRequired: "请先设置正式密码",
		ErrUserAlreadyExists:      "该手机号或邮箱已被注册",
		ErrUserNotFound:           "用户不存在",
		ErrMenuNotAvailable:       "菜单不可用",
		ErrRateLimited:            "请求过于频繁，服务器正在限流降级",
	},
	i18n.EnUS: {
		CodeSuccess:               "success",
		ErrSystemError:            "The server is busy, please try again later",
		ErrParamInvalid:           "Invalid request parameters",
		ErrUnauthorized:           "Unauthorized, please sign in first",
		ErrTokenExpired:           "Your session has expired, please sign in again",
		ErrForbidden:              "Access denied: insufficient permissions",
		ErrPasswordChangeRequired: "Please set your permanent password first",
		ErrUserAlreadyExists:      "The phone number or email is already registered",
		ErrUserNotFound:           "User not found",
		ErrMenuNotAvailable:       "Menu unavailable",
		ErrRateLimited:            "Too many requests, the server is rate limiting",
	},
}

// GetErrorMsg 根据错误码与语言返回对应的提示信息。
// 未注册的语言回退到 Default；未注册的错误码返回"未知业务错误"作为兜底。
func GetErrorMsg(code int, lang i18n.Lang) string {
	if lm, ok := codeMsgMap[lang]; ok {
		if msg, ok := lm[code]; ok {
			return msg
		}
	}
	if lm, ok := codeMsgMap[i18n.Default]; ok {
		if msg, ok := lm[code]; ok {
			return msg
		}
	}
	return "未知业务错误"
}
