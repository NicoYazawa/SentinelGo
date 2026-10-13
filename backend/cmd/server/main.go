// Package main 是 SentinelGo 后端服务的启动入口。
//
// 启动流程（按顺序）：
//  1. 初始化 slog 结构化日志
//  2. 加载 bootstrap.yaml 配置（支持热更新）
//  3. 初始化 JWT Secret
//  4. 连接数据库（GORM + 读写分离）
//  5. 连接 Redis（可选，失败不阻断启动，Token 黑名单等功能降级）
//  6. 初始化多级缓存（BigCache L1 + Redis L2）
//  7. 初始化 Snowflake 分布式 ID 生成器（WorkerID 从 Redis 自动分配）
//  8. 启动延时任务调度器与队列积压监控
//  9. 注册 GORM TraceID 传播插件
//  10. 自动迁移数据库表结构
//  11. 初始化 Casbin RBAC 权限引擎
//  12. Wire 依赖注入组装全链路组件
//  13. 初始化管理员账户、默认菜单、默认权限
//  14. 初始化多渠道登录服务（短信、GitHub OAuth）
//  15. 启动 HTTP 服务器并监听优雅关闭信号（SIGINT/SIGTERM）
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // Keep IANA time zones available on minimal Linux hosts.

	"sentinelgo/internal/model"
	"sentinelgo/internal/repository"
	"sentinelgo/internal/service"
	"sentinelgo/pkg/cache"
	"sentinelgo/pkg/casbin"
	"sentinelgo/pkg/config"
	"sentinelgo/pkg/database"
	"sentinelgo/pkg/idgen"
	myoauth "sentinelgo/pkg/oauth"
	"sentinelgo/pkg/queue"
	myredis "sentinelgo/pkg/redis"
	"sentinelgo/pkg/sms"
	"sentinelgo/pkg/token"

	"gorm.io/gorm"
)

func main() {
	// 1. 初始化 slog 结构化日志（JSON 格式，Info 级别，输出到 stdout）
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	// 2. 加载配置（支持热更新 Watch + 本地备份容灾）
	config.InitConfig("config/bootstrap.yaml")
	cfg := config.GlobalManager.Get()

	// 3. 初始化 JWT Secret（后续 Redis 就绪后会重新注入以启用黑名单）
	token.Init(cfg.JWT.Secret, nil)

	// 4. 初始化数据库（GORM + 读写分离 + 连接池配置）
	db := database.InitDatabase(database.DBConfig{
		MasterDSN:    cfg.Database.MasterDSN,
		SlaveDSNs:    cfg.Database.SlaveDSNs,
		MaxOpenConns: cfg.Database.MaxOpenConns,
		MaxIdleConns: cfg.Database.MaxIdleConns,
		MaxLifetime:  cfg.Database.MaxLifetime * time.Second,
		MaxIdleTime:  cfg.Database.MaxIdleTime * time.Second,
	})

	// 5. 初始化 Redis（可选，失败不阻断启动）
	//    使用匿名函数 + recover 确保 Redis 不可用时服务仍可启动
	func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Warn("Redis 初始化失败，Token 黑名单等功能将不可用", "error", r)
			}
		}()
		rdb := myredis.NewRedisClient(myredis.RedisConfig{
			Addr:         cfg.Redis.Addr,
			Password:     cfg.Redis.Password,
			DB:           cfg.Redis.DB,
			PoolSize:     cfg.Redis.PoolSize,
			MinIdleConns: cfg.Redis.MinIdleConns,
		})
		// 重新注入带 Redis 的 Token 管理器，启用黑名单功能
		token.Init(cfg.JWT.Secret, rdb)
	}()

	// 6. 初始化多级缓存（BigCache L1 + Redis L2 + Singleflight 防击穿）
	cache.Init(token.RDB)

	// 7. 初始化分布式 ID 生成器
	//    WorkerID 从 Redis 自动分配，不可用时降级为 0（单机模式）
	workerID, err := idgen.GetAutoWorkerID(token.RDB, cfg.App.Name)
	if err != nil {
		slog.Warn("WorkerID 自动分配失败，降级使用 0", "error", err)
		workerID = 0
	}
	if err := idgen.Init(workerID); err != nil {
		slog.Error("Snowflake ID 生成器初始化失败", "error", err)
		os.Exit(1)
	}

	// 8. 启动延时任务调度器与队列积压监控（依赖 Redis 可用）
	if token.RDB != nil {
		queue.StartDelayScheduler(token.RDB)
		// 监控已注册消费者的业务 Stream（与下方 StartConsume 保持一致）
		go queue.ReportMetrics(token.RDB, []string{
			sms.SMSStream,
		})
	}

	// 9. 注册 GORM TraceID 传播插件（将中间件 trace_id 注入 SQL 注释）
	database.RegisterTracePlugin(db)

	// 10. 自动迁移数据库表结构（GORM AutoMigrate，仅新增字段不删列）
	runMigrations(db)

	// 11. 初始化 Casbin RBAC 权限引擎（GORM 适配器 + rbac_model.conf）
	casbin.Init(db, "config/rbac_model.conf")

	// 12. Wire 依赖注入：Repository → Service → Handler → Router
	router := InitComponents(db)

	// 13. 初始化管理员账户、默认菜单、默认权限（幂等操作）
	initAdminUser(db)
	initDefaultMenus(db)
	initDefaultPermissions(db)

	// 14. 初始化多渠道登录服务（短信验证码 + GitHub OAuth）
	//     按配置选择短信下发通道：dev（仅日志）或 aliyun（真实网关）。
	var smsSender sms.Sender = sms.NewDevSender()
	if cfg.SMS.Provider == "aliyun" {
		aliyunSender, aerr := sms.NewAliyunDypnsSender(cfg.SMS.SignName, cfg.SMS.TemplateCode, cfg.SMS.CodeTTL)
		if aerr != nil {
			panic(fmt.Errorf("阿里云短信初始化失败，拒绝降级为开发通道: %w", aerr))
		} else {
			smsSender = aliyunSender
		}
	}
	smsSvc := sms.NewSMSService(token.RDB, smsSender, sms.Config{
		CodeTTL:     cfg.SMS.CodeTTL,
		RateLimit:   cfg.SMS.RateLimit,
		MaxAttempts: cfg.SMS.MaxAttempts,
	})
	var githubOAuth *myoauth.GitHubOAuth
	if cfg.OAuth.GitHub.ClientID != "" {
		githubOAuth = myoauth.NewGitHubOAuth(
			cfg.OAuth.GitHub.ClientID,
			cfg.OAuth.GitHub.ClientSecret,
			cfg.OAuth.GitHub.RedirectURI,
			cfg.OAuth.GitHub.FrontendURL,
			cfg.OAuth.GitHub.HTTPProxy,
		)
	}
	service.InitMultiAuth(smsSvc, githubOAuth)

	// 14b. 启动异步短信下发消费者（依赖 Redis 可用）。
	// SendCode 在 Producer 就绪后改为将「发送任务」投递到 Redis Stream，
	// 由本消费者协程调用真实 Sender 异步下发，解耦接口响应与短信网关延迟。
	if token.RDB != nil {
		producer := queue.NewProducer(token.RDB)
		smsSvc.SetProducer(producer)

		smsConsumer := queue.NewConsumer(token.RDB, sms.SMSStream, "sms_workers")
		smsConsumer.StartConsume(func(payload map[string]any) error {
			phone, _ := payload["phone"].(string)
			code, _ := payload["code"].(string)
			if phone == "" || code == "" {
				return nil // 脏数据直接丢弃，不阻塞队列
			}
			return smsSender.Send(context.Background(), phone, code)
		})
	}

	// 15. 启动 HTTP 服务器
	addr := fmt.Sprintf(":%d", cfg.App.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	// 异步启动，主 goroutine 等待退出信号
	go func() {
		slog.Info("服务启动", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("服务启动失败", "error", err)
			os.Exit(1)
		}
	}()

	// 16. 等待系统信号（SIGINT Ctrl+C / SIGTERM kill）
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// 17. 优雅关闭（最长等待 10 秒，超时强制退出）
	slog.Info("收到关闭信号，开始优雅退出...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("服务关闭异常", "error", err)
		os.Exit(1)
	}

	slog.Info("服务已安全退出")
}

// runMigrations 执行 GORM AutoMigrate，自动创建/更新表结构（仅新增字段，不删除已有列）。
func runMigrations(db *gorm.DB) {
	if err := db.AutoMigrate(
		&model.User{},
		&model.Role{},
		&model.Menu{},
		&model.Permission{},
		&model.CasbinRule{},
		&model.OperationLog{},
		&model.UserRole{},
		&model.RoleMenu{},
		&model.UserAuth{},
		&model.AuthSession{},
		&model.OAuthState{},
	); err != nil {
		panic(fmt.Errorf("数据库迁移失败，拒绝启动: %w", err))
	}

	// 时区字段回填：已存在用户的 timezone 为空时，统一设为 Asia/Shanghai（中国用户默认）。
	if err := db.Exec("UPDATE sys_user SET timezone = 'Asia/Shanghai' WHERE timezone IS NULL OR timezone = ''").Error; err != nil {
		slog.Warn("回填用户时区默认值失败", "error", err)
	}
	// 语言字段回填：已存在用户的 locale 为空时，统一设为 zh-CN。
	if err := db.Exec("UPDATE sys_user SET locale = 'zh-CN' WHERE locale IS NULL OR locale = ''").Error; err != nil {
		slog.Warn("回填用户语言默认值失败", "error", err)
	}
}

// initAdminUser 初始化管理员账户（admin/admin，ID=1，角色 admin 和 user）。
// 如果管理员已存在则跳过，保证幂等。
func initAdminUser(db *gorm.DB) {
	repo := repository.NewUserRepository(db)
	roleRepo := repository.NewRoleRepository(db)
	authSvc := service.NewAuthService(repo, roleRepo, db)
	if err := authSvc.InitAdmin(); err != nil {
		panic(fmt.Errorf("初始化管理员失败，拒绝启动: %w", err))
	}
}

// initDefaultMenus 初始化系统默认菜单结构（8 个菜单项），并分配给 admin 角色。
// 使用 FirstOrCreate 保证幂等，重复调用不会产生重复数据。
func initDefaultMenus(db *gorm.DB) {
	menuRepo := repository.NewMenuRepository(db)
	roleRepo := repository.NewRoleRepository(db)
	menuSvc := service.NewMenuService(menuRepo, roleRepo, db)
	if err := menuSvc.InitDefaultMenus(); err != nil {
		slog.Warn("初始化默认菜单失败", "error", err)
	}
}

// initDefaultPermissions 初始化默认权限定义（16 条 CRUD 权限），仅在表为空时执行。
func initDefaultPermissions(db *gorm.DB) {
	permRepo := repository.NewPermissionRepository(db)
	permSvc := service.NewPermissionService(permRepo, db)
	if err := permSvc.InitDefaultPermissions(); err != nil {
		slog.Warn("初始化默认权限失败", "error", err)
	}
}
