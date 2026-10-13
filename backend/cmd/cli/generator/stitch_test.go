package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The subprocess is a local stand-in for the Wire executable. It records its
// invocation so the test verifies our CLI boundary without invoking a real build.
func TestMain(m *testing.M) {
	if os.Getenv("SENTINEL_WIRE_HELPER") == "1" {
		if len(os.Args) != 2 || os.Args[1] != "./cmd/server" {
			os.Exit(9)
		}
		if err := os.WriteFile("wire-invoked", []byte(os.Args[1]), 0600); err != nil {
			os.Exit(8)
		}
		if os.Getenv("SENTINEL_WIRE_FAIL") == "1" {
			fmt.Fprintln(os.Stderr, "wire failed")
			os.Exit(7)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func wireFixture(t *testing.T) string {
	t.Helper()
	t.Chdir(t.TempDir())
	if err := os.MkdirAll("cmd/server", 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("go.mod", []byte("module stitchtest\n\ngo 1.25.5\n"), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("cmd", "server", "wire.go")
	text := "package main\nvar (\nRepositorySet = wire.NewSet(\n)\nServiceSet = wire.NewSet(\n)\nHandlerSet = wire.NewSet(\n)\n)\n"
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAppendToWireSetAddsEveryLayerOnce(t *testing.T) {
	path := wireFixture(t)
	data := TemplateData{CamelName: "OrderItem", LowerCamelName: "orderItem"}
	if err := appendToWireSet(data); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"repository.NewOrderItemRepository", "service.NewOrderItemService", "handler.NewOrderItemHandler"} {
		if strings.Count(string(first), name) != 1 {
			t.Errorf("Provider %s should occur once: %s", name, first)
		}
	}
	if err := appendToWireSet(data); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("repeated stitching changed providers")
	}
}

func TestAppendToWireSetReadFailure(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := appendToWireSet(TemplateData{CamelName: "Order"}); !os.IsNotExist(err) {
		t.Fatalf("want missing wire.go error, got %v", err)
	}
}

func TestAutoStitchToolInvocation(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			path := wireFixture(t)
			bin := t.TempDir()
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			bytes, err := os.ReadFile(executable)
			if err != nil {
				t.Fatal(err)
			}
			wireName := "wire"
			if strings.HasSuffix(executable, ".exe") {
				wireName += ".exe"
			}
			if err := os.WriteFile(filepath.Join(bin, wireName), bytes, 0755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("SENTINEL_WIRE_HELPER", "1")
			if fail {
				t.Setenv("SENTINEL_WIRE_FAIL", "1")
			} else {
				t.Setenv("SENTINEL_WIRE_FAIL", "0")
			}
			autoStitch(TemplateData{CamelName: "OrderItem"})
			invoked, err := os.ReadFile("wire-invoked")
			if err != nil || string(invoked) != "./cmd/server" {
				t.Fatalf("Wire not invoked with expected target: %q %v", invoked, err)
			}
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"repository.NewOrderItemRepository", "service.NewOrderItemService", "handler.NewOrderItemHandler"} {
				if !strings.Contains(string(content), name) {
					t.Fatalf("stitch omitted %s", name)
				}
			}
		})
	}
}

func TestGenerateModuleCannotCreateDirectory(t *testing.T) {
	base := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(base, []byte("blocked"), 0600); err != nil {
		t.Fatal(err)
	}
	oldBase, oldStitch := GenBaseDir, RunStitch
	GenBaseDir, RunStitch = base, false
	t.Cleanup(func() { GenBaseDir, RunStitch = oldBase, oldStitch })
	err := GenerateModule(TemplateData{CamelName: "Order", ModulePackage: "order"})
	if err == nil || !strings.Contains(err.Error(), "创建目录失败") {
		t.Fatalf("expected directory failure, got %v", err)
	}
	content, err := os.ReadFile(base)
	if err != nil || string(content) != "blocked" {
		t.Fatal("failed generation modified existing file")
	}
}

func TestRenderLayerExecutionError(t *testing.T) {
	if _, err := renderLayer("bad", "{{.MissingField}}", TemplateData{}); err == nil || !strings.Contains(err.Error(), "渲染 bad 模板失败") {
		t.Fatalf("expected render error, got %v", err)
	}
}
