//go:build wireinject
// +build wireinject

// Package main 提供 Google Wire 编译时依赖注入的定义文件。
//
// Wire 是 Google 开源的编译时依赖注入工具，通过代码生成替代运行时反射。
// 本文件仅在 wireinject 构建标签下编译，用于指导 wire 生成 wire_gen.go。
//
// 依赖链组装顺序：
//
//	DB → Repository → Service → Handler → Router → gin.Engine
//
// 三层 ProviderSet 分别对应数据访问层、业务逻辑层、HTTP 控制层，
// Wire 会自动解析依赖关系并按拓扑顺序调用 Provider 函数。
package main

import (
	"github.com/gin-gonic/gin"
	"github.com/google/wire"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/gorm"

	"sentinelgo/internal/handler"
	"sentinelgo/internal/middleware"
	"sentinelgo/internal/repository"
	"sentinelgo/internal/service"
	"sentinelgo/pkg/response"
)

// RepositorySet 数据访问层所有 Provider（GORM CRUD 实现）。
var RepositorySet = wire.NewSet(
	repository.NewUserRepository,
	repository.NewRoleRepository,
	repository.NewMenuRepository,
	repository.NewPermissionRepository,
	repository.NewOperationLogRepository,
	repository.NewDashboardRepository,
)

// ServiceSet 业务逻辑层所有 Provider。
var ServiceSet = wire.NewSet(
	service.NewAuthService,
	service.NewUserService,
	service.NewRoleService,
	service.NewMenuService,
	service.NewPermissionService,
	service.NewOperationLogService,
	service.NewDashboardService,
)

// HandlerSet HTTP 控制层所有 Provider。
var HandlerSet = wire.NewSet(
	handler.NewAuthHandler,
	handler.NewUserHandler,
	handler.NewRoleHandler,
	handler.NewMenuHandler,
	handler.NewPermissionHandler,
	handler.NewOperationLogHandler,
	handler.NewDashboardHandler,
)

// InitComponents 是 Wire 注入入口函数。
// Wire 会根据此函数的 wire.Build 调用，自动生成 wire_gen.go 中的实现。
func InitComponents(db *gorm.DB) *gin.Engine {
	wire.Build(
		RepositorySet,
		ServiceSet,
		HandlerSet,
		NewRouter,
	)
	return nil
}

// NewRouter 创建并配置 Gin 路由引擎。
//
// 中间件链（按顺序执行）：
//  1. Recovery — 全局 Panic 捕获，返回结构化 500 JSON
//  2. Trace — 注入 TraceID 到 Context 和 Response Header
//  3. Cors — 跨域处理（OPTIONS 预检 204）
//  4. Logger — 结构化请求日志（slog + TraceID + 耗时）
//  5. Metrics — Prometheus HTTP 指标采集（Counter + Histogram）
//  6. RateLimit — 令牌桶限流（全局 5000/s + 每 IP 50/s）
//
// 路由分组：
//   - /metrics — Prometheus 指标暴露端点
//   - /healthz — 健康检查（始终返回 ok）
//   - /readyz — 就绪检查（验证 DB 连接）
//   - /api/v1/auth/* — 公开认证接口（登录/注册/短信/OAuth/找回密码）
//   - /api/v1/* — JWT 认证接口（需 Bearer Token）
func NewRouter(
	authHandler *handler.AuthHandler,
	userHandler *handler.UserHandler,
	roleHandler *handler.RoleHandler,
	menuHandler *handler.MenuHandler,
	permHandler *handler.PermissionHandler,
	opLogHandler *handler.OperationLogHandler,
	dashHandler *handler.DashboardHandler,
	db *gorm.DB,
) *gin.Engine {
	r := gin.New()

	// 操作审计日志仓储（供审计中间件写入）
	opLogRepo := repository.NewOperationLogRepository(db)

	// 全局中间件链（按顺序执行）
	r.Use(middleware.Recovery())
	r.Use(middleware.I18n()) // 语言协商：尽早解析，供后续中间件/Handler 取用
	r.Use(middleware.Trace())
	r.Use(middleware.Cors())
	r.Use(middleware.Logger())
	r.Use(middleware.Metrics())
	r.Use(middleware.RateLimit())

	// Prometheus 指标端点（由 promhttp 自动注册 Go/Process 收集器）
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// 健康检查：K8s liveness probe
	r.GET("/healthz", func(c *gin.Context) {
		response.Success(c, gin.H{"status": "ok"})
	})

	// 就绪检查：K8s readiness probe（验证 DB 连接）
	r.GET("/readyz", func(c *gin.Context) {
		if err := db.Exec("SELECT 1").Error; err != nil {
			response.Fail(c, 503, "DB disconnected")
			return
		}
		response.Success(c, gin.H{"status": "ready", "db": "connected"})
	})

	// ---- 公开接口（无需认证） ----

	// 兼容旧版 API 路径
	r.POST("/api/login", authHandler.Login)
	r.POST("/api/register", authHandler.Register)

	// Token 刷新（使用 Refresh Token，无需 Access Token）
	r.POST("/api/v1/auth/refresh", authHandler.Refresh)
	r.GET("/api/v1/auth/bootstrap/status", authHandler.BootstrapStatus)

	// 多渠道登录（审计登录/注册安全事件）
	r.POST("/api/v1/auth/login", middleware.NewAuditMiddleware(opLogRepo), authHandler.Login)
	r.POST("/api/v1/auth/register", middleware.NewAuditMiddleware(opLogRepo), authHandler.Register)

	// 短信验证码登录
	r.POST("/api/v1/auth/sms/send", authHandler.SendSMS)
	r.POST("/api/v1/auth/sms/login", authHandler.SMSLogin)

	// GitHub OAuth 第三方登录
	r.GET("/api/v1/auth/oauth/github", authHandler.GitHubOAuth)
	r.GET("/api/v1/auth/oauth/github/callback", authHandler.GitHubCallback)

	// 找回密码（通过短信验证码）
	r.POST("/api/v1/auth/password/forgot", authHandler.ForgotPassword)
	r.POST("/api/v1/auth/password/reset", authHandler.ResetPassword)

	// 静态文件服务（头像等上传资源）
	r.Static("/uploads", "./uploads")

	// ---- 认证接口（需要 JWT Bearer Token） ----
	v1 := r.Group("/api/v1")
	v1.Use(middleware.JWTAuth(db), middleware.NewAuditMiddleware(opLogRepo))
	{
		// 当前用户信息与个人中心
		v1.GET("/user/info", userHandler.GetCurrentUser)
		v1.GET("/user/profile", userHandler.GetProfile)
		v1.PUT("/user/profile", userHandler.UpdateProfile)
		v1.POST("/user/avatar", userHandler.UploadAvatar)

		// 认证相关（登出、渠道绑定）
		v1.POST("/auth/logout", authHandler.Logout)
		v1.POST("/auth/password/initialize", authHandler.InitializePassword)
		v1.GET("/auth/channels", authHandler.GetChannels)
		v1.POST("/auth/bind/phone", authHandler.BindPhone)
		v1.POST("/auth/unbind", authHandler.Unbind)

		// 系统监控
		v1.GET("/monitor/stats", handler.NewMonitorHandler().Stats)

		// 操作审计日志查询（仅管理员可访问）
		v1.GET("/audit/logs", opLogHandler.List)

		// 首页仪表盘聚合统计（KPI + 安全审计看板）
		v1.GET("/dashboard/stats", dashHandler.Stats)

		// 用户菜单（根据角色过滤）
		v1.GET("/user/menus", menuHandler.GetUserMenus)

		// 用户管理 CRUD
		users := v1.Group("/users")
		{
			users.GET("", userHandler.List)
			users.GET("/:id", userHandler.Get)
			users.POST("", userHandler.Create)
			users.PUT("/:id", userHandler.Update)
			users.DELETE("/:id", userHandler.Delete)
			users.PUT("/:id/status", userHandler.UpdateStatus)
			users.PUT("/:id/password", userHandler.UpdatePassword)
			users.GET("/:id/roles", userHandler.GetRoles)
			users.PUT("/:id/roles", userHandler.AssignRoles)
		}

		// 角色管理 CRUD + 菜单分配 + 权限分配
		roles := v1.Group("/roles")
		{
			roles.GET("", roleHandler.List)
			roles.GET("/:id", roleHandler.Get)
			roles.POST("", roleHandler.Create)
			roles.PUT("/:id", roleHandler.Update)
			roles.DELETE("/:id", roleHandler.Delete)
			roles.PUT("/:id/status", roleHandler.UpdateStatus)
			roles.GET("/:id/menus", roleHandler.GetMenus)
			roles.PUT("/:id/menus", roleHandler.AssignMenus)
			roles.PUT("/:id/permissions", roleHandler.AssignPermissions)
		}

		// 菜单管理 CRUD
		menus := v1.Group("/menus")
		{
			menus.GET("", menuHandler.List)
			menus.GET("/tree", menuHandler.List)
			menus.GET("/:id", menuHandler.Get)
			menus.POST("", menuHandler.Create)
			menus.PUT("/:id", menuHandler.Update)
			menus.DELETE("/:id", menuHandler.Delete)
			menus.PUT("/:id/status", menuHandler.UpdateStatus)
		}

		// 权限定义管理 CRUD
		permissions := v1.Group("/permissions")
		{
			permissions.GET("", permHandler.List)
			permissions.GET("/:id", permHandler.Get)
			permissions.POST("", permHandler.Create)
			permissions.PUT("/:id", permHandler.Update)
			permissions.DELETE("/:id", permHandler.Delete)
		}
	}

	return r
}
