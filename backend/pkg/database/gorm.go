// Package database 提供 GORM 数据库初始化工厂，支持读写分离、连接池调优、慢查询日志。
//
// 核心能力：
//   - 基于 GORM + MySQL 驱动的数据库连接初始化
//   - dbresolver 插件实现主库写入、从库读取的读写分离
//   - 可配置连接池参数（最大连接数、空闲连接数、连接生命周期）
//   - 集成 slog 结构化日志桥接器，自动记录慢查询与 SQL 错误
//   - TraceID 传播插件，打通 HTTP → GORM 的全链路追踪
package database

import (
	"context"
	"fmt"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"
)

// DBConfig 是初始化数据库连接所需的配置参数。
type DBConfig struct {
	MasterDSN    string        // 主库（写库）DSN 连接字符串
	SlaveDSNs    []string      // 从库（读库）DSN 列表，为空则不启用读写分离
	MaxOpenConns int           // 连接池最大打开连接数
	MaxIdleConns int           // 连接池最大空闲连接数
	MaxLifetime  time.Duration // 单个连接的最大存活时间，超时后自动关闭
	MaxIdleTime  time.Duration // 空闲连接的最大存活时间，超时后自动关闭
}

// InitDatabase 初始化 MySQL 数据库连接，配置连接池与读写分离。
//
// 初始化步骤：
//  1. 使用 MySQL 驱动连接主库
//  2. 注入 slog 日志桥接器（慢查询阈值为 200ms）
//  3. 开启 PrepareStmt 预编译缓存，提升 SQL 执行效率
//  4. 若配置了从库 DSN，通过 dbresolver 插件建立读写分离路由
//  5. 配置连接池参数（MaxOpenConns, MaxIdleConns, MaxLifetime, MaxIdleTime）
//  6. Ping 验证数据库可达性
//
// 任何步骤失败都会 panic，采用 fail-fast 策略。
func InitDatabase(cfg DBConfig) *gorm.DB {
	// 第一步：连接主库，配置日志与预编译缓存
	db, err := gorm.Open(mysql.Open(cfg.MasterDSN), &gorm.Config{
		Logger:      NewZapLogger(200 * time.Millisecond), // 超过 200ms 的 SQL 记为慢查询
		PrepareStmt: true,                                 // 开启预编译语句缓存，减少 SQL 解析开销
	})
	if err != nil {
		panic(fmt.Sprintf("数据库连接失败: %v", err))
	}

	// 第二步：配置读写分离（仅在配置了从库时启用）
	if len(cfg.SlaveDSNs) > 0 {
		replicas := make([]gorm.Dialector, len(cfg.SlaveDSNs))
		for i, dsn := range cfg.SlaveDSNs {
			replicas[i] = mysql.Open(dsn)
		}
		// dbresolver 使用 RandomPolicy 随机选择从库，实现负载均衡
		if err := db.Use(dbresolver.Register(dbresolver.Config{
			Sources:  []gorm.Dialector{mysql.Open(cfg.MasterDSN)}, // 写操作走主库
			Replicas: replicas,                                    // 读操作随机分配到从库
			Policy:   dbresolver.RandomPolicy{},
		})); err != nil {
			panic(fmt.Sprintf("配置读写分离失败: %v", err))
		}
	}

	// 第三步：获取底层 *sql.DB，配置连接池参数
	sqlDB, err := db.DB()
	if err != nil {
		panic(err)
	}

	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.MaxLifetime)
	// MaxIdleTime 未配置时默认 15 分钟，防止空闲连接被 MySQL 服务端主动断开
	if cfg.MaxIdleTime > 0 {
		sqlDB.SetConnMaxIdleTime(cfg.MaxIdleTime)
	} else {
		sqlDB.SetConnMaxIdleTime(15 * time.Minute)
	}

	// 第四步：验证连接可用性
	if err := sqlDB.Ping(); err != nil {
		panic(fmt.Sprintf("数据库 Ping 失败: %v", err))
	}

	return db
}

// GetDB 从 context.Context 中派生带 TraceID 的 GORM 实例。
// 用于在业务层快速获取链路追踪版本的数据库连接。
func GetDB(ctx context.Context, db *gorm.DB) *gorm.DB {
	return db.WithContext(ctx)
}
