package generator

import (
	"bytes"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
)

func TestNameConversions(t *testing.T) {
	cases := []struct {
		in, camel, lower, snake, kebab string
	}{
		{"OrderItem", "OrderItem", "orderItem", "order_item", "order-item"},
		{"User", "User", "user", "user", "user"},
		{"", "", "", "", ""},
	}
	for _, c := range cases {
		if got := ToCamelCase(c.in); got != c.camel {
			t.Errorf("ToCamelCase(%q)=%q want %q", c.in, got, c.camel)
		}
		if got := ToLowerCamelCase(c.in); got != c.lower {
			t.Errorf("ToLowerCamelCase(%q)=%q want %q", c.in, got, c.lower)
		}
		if got := ToSnakeCase(c.in); got != c.snake {
			t.Errorf("ToSnakeCase(%q)=%q want %q", c.in, got, c.snake)
		}
		if got := ToKebabCase(c.in); got != c.kebab {
			t.Errorf("ToKebabCase(%q)=%q want %q", c.in, got, c.kebab)
		}
	}
}

func TestTemplateParse(t *testing.T) {
	if _, err := templateParse("m", "{{.CamelName}}"); err != nil {
		t.Errorf("合法模板不应报错: %v", err)
	}
	if _, err := templateParse("m", "{{ .Bad "); err == nil {
		t.Error("坏模板应报错")
	}
}

func TestTemplateGetters(t *testing.T) {
	data := TemplateData{TableName: "order_items", CamelName: "OrderItem", LowerCamelName: "orderItem", ModulePackage: "orderitem"}
	for layer, tmpl := range map[string]*template.Template{"model": modelTemplate(), "repository": repositoryTemplate(), "service": serviceTemplate(), "handler": handlerTemplate()} {
		t.Run(layer, func(t *testing.T) {
			var out bytes.Buffer
			if err := tmpl.Execute(&out, data); err != nil {
				t.Fatal(err)
			}
			parsed, err := parser.ParseFile(token.NewFileSet(), layer+".go", out.Bytes(), parser.AllErrors)
			if err != nil {
				t.Fatalf("渲染代码语法错误: %v", err)
			}
			if parsed.Name.Name != layer || !strings.Contains(out.String(), "OrderItem") || strings.Contains(out.String(), "{{") {
				t.Fatalf("模板未正确渲染: %s", out.String())
			}
		})
	}
}

func TestRenderLayerError(t *testing.T) {
	if _, err := renderLayer("x", "{{ .Bad ", TemplateData{}); err == nil {
		t.Error("坏模板应报错")
	}
}

func TestGenerateModuleToTemp(t *testing.T) {
	dir := t.TempDir()
	oldD, oldS := GenBaseDir, RunStitch
	GenBaseDir = dir
	RunStitch = false
	defer func() { GenBaseDir = oldD; RunStitch = oldS }()

	data := TemplateData{
		TableName:      "order_items",
		CamelName:      "OrderItem",
		LowerCamelName: "orderItem",
		ModulePackage:  "orderitem",
	}
	if err := GenerateModule(data); err != nil {
		t.Fatal(err)
	}
	contents := map[string][]byte{}
	for _, layer := range []string{"model", "repository", "service", "handler"} {
		p := filepath.Join(dir, layer, "orderitem.go")
		content, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := parser.ParseFile(token.NewFileSet(), p, content, parser.AllErrors); err != nil {
			t.Fatalf("生成代码语法错误: %v", err)
		}
		if !strings.Contains(string(content), "OrderItem") {
			t.Fatalf("生成代码缺少实体名: %s", p)
		}
		contents[p] = append(content, []byte("\n// preserve custom changes\n")...)
		if err := os.WriteFile(p, contents[p], 0600); err != nil {
			t.Fatal(err)
		}
	}
	// 再次生成应跳过已存在文件（不报错）
	if err := GenerateModule(data); err != nil {
		t.Fatal(err)
	}
	for p, want := range contents {
		got, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("重复生成覆盖了现有文件: %s", p)
		}
	}
}
