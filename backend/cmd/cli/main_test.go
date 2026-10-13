package main

import (
	"os"
	"path/filepath"
	"testing"

	"sentinelgo/cmd/cli/generator"
)

func TestRunMissingParams(t *testing.T) {
	if err := run([]string{"gen"}); err == nil {
		t.Error("缺少参数应返回错误")
	}
	if err := run([]string{"gen", "--table", "users"}); err == nil {
		t.Error("仅指定 table 缺 module 应报错")
	}
	if err := run([]string{"gen", "--module", "user"}); err == nil {
		t.Error("仅指定 module 缺 table 应报错")
	}
}

func TestRunGenToTemp(t *testing.T) {
	dir := t.TempDir()
	oldD, oldS := generator.GenBaseDir, generator.RunStitch
	generator.GenBaseDir = dir
	generator.RunStitch = false
	defer func() { generator.GenBaseDir = oldD; generator.RunStitch = oldS }()

	if err := run([]string{"gen", "--table", "order_items", "--module", "orderitem"}); err != nil {
		t.Fatal(err)
	}
	for _, layer := range []string{"model", "repository", "service", "handler"} {
		p := filepath.Join(dir, layer, "orderitem.go")
		if _, err := os.Stat(p); err != nil {
			t.Errorf("未生成 %s: %v", p, err)
		}
	}
}

func TestRunNoSubcommand(t *testing.T) {
	// 无子命令时 kong 要求 gen 子命令，应返回错误
	if err := run([]string{}); err == nil {
		t.Error("无子命令应报错（kong 要求 gen）")
	}
}
