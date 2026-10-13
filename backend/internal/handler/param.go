package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"sentinelgo/pkg/response"
)

// contextKeyUserID 是 JWTAuth 中间件注入当前登录用户 ID 使用的 Gin context key。
const contextKeyUserID = "current_user_id"

// parseIDParam 解析路径中的 uint64 参数。
// 解析失败时统一返回参数错误响应并返回 false，避免 id 静默退化为 0。
func parseIDParam(c *gin.Context, name string) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil {
		response.Fail(c, response.ErrParamInvalid, "路径参数无效: "+name)
		return 0, false
	}
	return id, true
}

// currentUserID 读取 JWTAuth 中间件注入的当前登录用户 ID。
// 缺失或类型不符时统一返回未授权响应，避免 nil 断言后静默取到 0。
func currentUserID(c *gin.Context) (uint64, bool) {
	v, ok := c.Get(contextKeyUserID)
	if !ok {
		response.Fail(c, response.ErrUnauthorized, "请先登录")
		return 0, false
	}
	id, ok := v.(uint64)
	if !ok {
		response.Fail(c, response.ErrUnauthorized, "登录状态无效，请重新登录")
		return 0, false
	}
	return id, true
}
