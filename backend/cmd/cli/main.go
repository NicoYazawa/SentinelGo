// Package main 是 SentinelGo CLI 脚手架工具的入口。
//
// sentinel-cli 提供代码生成命令，可一键生成标准分层核心业务代码：
//
//	sentinel-cli gen --table users --module user
//
// 生成的文件分布在四层：
//   - internal/model/user.go         — GORM 实体定义
//   - internal/repository/user.go    — 数据访问层
//   - internal/service/user.go       — 业务逻辑层
//   - internal/handler/user.go       — HTTP 控制器
//
// 生成后会自动 go fmt 格式化代码并运行 wire 重新生成依赖注入图谱。
package main

import (
	"fmt"
	"os"

	"sentinelgo/cmd/cli/generator"

	"github.com/alecthomas/kong"
)

// cli 定义 sentinel-cli 的命令结构（由 kong 解析）。
var cli struct {
	Gen struct {
		Table  string `short:"t" help:"数据库表名 (如: users)"`
		Module string `short:"m" help:"生成的模块包名 (如: user)"`
	} `cmd:"" help:"一键生成标准分层核心业务逻辑代码"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		// 错误信息已由 run 或 kong 输出到 stderr
		os.Exit(1)
	}
}

// run 解析并执行 CLI 命令，便于单元测试。
func run(args []string) error {
	parser, err := kong.New(&cli,
		kong.Name("sentinel-cli"),
		kong.Description("SentinelGo 通用网站底座工业级脚手架生成工具"),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "解析命令行失败:", err)
		return err
	}

	ctx, err := parser.Parse(args)
	if err != nil {
		// kong 已将错误信息输出到 stderr
		return err
	}

	switch ctx.Command() {
	case "gen":
		if cli.Gen.Table == "" || cli.Gen.Module == "" {
			fmt.Println("错误: 必须指定 --table 和 --module 参数")
			return fmt.Errorf("缺少必要参数 --table / --module")
		}

		// 表名 → 驼峰命名转换
		camel := generator.ToCamelCase(cli.Gen.Table)
		lowerCamel := generator.ToLowerCamelCase(cli.Gen.Table)

		data := generator.TemplateData{
			TableName:      cli.Gen.Table,
			CamelName:      camel,
			LowerCamelName: lowerCamel,
			ModulePackage:  cli.Gen.Module,
		}

		// 执行代码生成
		if err := generator.GenerateModule(data); err != nil {
			fmt.Printf("代码生成失败: %v\n", err)
			return err
		}
	}
	return nil
}
