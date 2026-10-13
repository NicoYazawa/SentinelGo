// Package generator 提供 SentinelGo 代码生成器的核心逻辑。
//
// 代码生成器根据数据库表名自动生成标准四层架构代码：
//   - model      — GORM 实体（表名映射 + 基础字段）
//   - repository — 数据访问层（CRUD 封装）
//   - service    — 业务逻辑层（透传 Repository）
//   - handler    — HTTP 控制器（Gin 路由处理）
//
// 生成后自动执行：go fmt → wire 重新生成依赖注入图谱。
package generator

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// TemplateData 灌入模板的元数据，由 CLI 参数解析后填充。
type TemplateData struct {
	TableName      string // 数据库表名（snake_case），如 order_items
	CamelName      string // 大驼峰命名，如 OrderItem
	LowerCamelName string // 小驼峰命名，如 orderItem
	ModulePackage  string // 生成的模块目录名，如 orderitem
}

// GenBaseDir 是代码生成的目标根目录，默认 "internal"。
// 测试时可临时指向临时目录，避免污染真实仓库。
var GenBaseDir = "internal"

// RunStitch 控制生成后是否自动执行 go fmt 与 wire 缝合。
// 测试时置为 false 可跳过外部命令调用。
var RunStitch = true

// GenerateModule 生成模块的四层代码文件。
//
// 生成流程：
//  1. 检查目标文件是否已存在（存在则跳过，防止覆盖手写代码）
//  2. 解析 Go template 模板
//  3. 渲染模板并写入目标路径
//  4. 自动格式化代码并重新生成 Wire 依赖图谱（受 RunStitch 控制）
func GenerateModule(data TemplateData) error {
	baseDir := GenBaseDir
	// 四层代码文件的目标路径映射
	filesToGen := map[string]string{
		"model":      filepath.Join(baseDir, "model", data.ModulePackage+".go"),
		"repository": filepath.Join(baseDir, "repository", data.ModulePackage+".go"),
		"service":    filepath.Join(baseDir, "service", data.ModulePackage+".go"),
		"handler":    filepath.Join(baseDir, "handler", data.ModulePackage+".go"),
	}

	templates := map[string]string{
		"model":      modelTemplateStr,
		"repository": repositoryTemplateStr,
		"service":    serviceTemplateStr,
		"handler":    handlerTemplateStr,
	}

	for layer, targetPath := range filesToGen {
		// 文件已存在则跳过，防止覆盖已有代码
		if _, err := os.Stat(targetPath); err == nil {
			fmt.Printf("[警告] 文件已存在，跳过生成: %s\n", targetPath)
			continue
		}

		buf, err := renderLayer(layer, templates[layer], data)
		if err != nil {
			return err
		}

		// 确保目标目录存在
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("创建目录失败: %w", err)
		}

		if err := os.WriteFile(targetPath, buf, 0644); err != nil {
			return fmt.Errorf("写入 %s 文件失败: %w", layer, err)
		}
		fmt.Printf("[成功] 已生成 %s 层代码: %s\n", strings.ToUpper(layer), targetPath)
	}

	// 自动缝合到依赖注入体系（受 RunStitch 控制）
	if RunStitch {
		autoStitch(data)
	}
	return nil
}

// renderLayer 解析并渲染单个模板，返回渲染后的字节内容。
func renderLayer(layer, tmplStr string, data TemplateData) ([]byte, error) {
	tmpl, err := templateParse(layer, tmplStr)
	if err != nil {
		return nil, fmt.Errorf("解析 %s 模板失败: %w", layer, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("渲染 %s 模板失败: %w", layer, err)
	}
	return buf.Bytes(), nil
}

// autoStitch 自动格式化代码并重新生成 Wire 依赖图谱。
//
// 执行步骤：
//  1. 将新模块的 Provider 追加到 wire.go 的对应 Set 中
//  2. go fmt 格式化所有代码
//  3. wire 重新生成 cmd/server/wire_gen.go
func autoStitch(data TemplateData) {
	fmt.Println("正在自动格式化代码并缝合依赖链...")

	if err := appendToWireSet(data); err != nil {
		fmt.Printf("[警告] 自动追加 Wire Provider 失败: %v\n", err)
		fmt.Println("[提示] 请手动将新 Provider 添加到 cmd/server/wire.go 的对应 Set 中")
	}

	if err := exec.Command("go", "fmt", "./...").Run(); err != nil {
		fmt.Printf("[警告] go fmt 执行失败: %v\n", err)
	}

	// 运行 wire 重新生成依赖注入代码
	wireCmd := exec.Command("wire", "./cmd/server")
	var stderr bytes.Buffer
	wireCmd.Stderr = &stderr
	if err := wireCmd.Run(); err != nil {
		fmt.Printf("[失败] Wire 自动依赖注入失败: %s\n", stderr.String())
		fmt.Println("[提示] 请手动检查 cmd/server/wire.go 是否有循环依赖或未注册组件。")
		return
	}

	fmt.Println("[完美] 全链路依赖图谱已自动重新生成完毕！新模块已可直接拉起运行。")
}

// appendToWireSet 在 wire.go 中自动追加新模块的 Provider 到对应的 Set。
//
// 使用正则表达式匹配三个 ProviderSet 的闭合括号位置，
// 在其前插入新的 Provider 函数调用。如果 Provider 已存在则跳过。
func appendToWireSet(data TemplateData) error {
	wirePath := "cmd/server/wire.go"
	content, err := os.ReadFile(wirePath)
	if err != nil {
		return err
	}
	text := string(content)

	camel := data.CamelName
	lcamel := data.LowerCamelName

	// 三层 ProviderSet 的正则替换规则
	replacements := map[string]struct {
		pattern  string
		insert   string
		provider string
	}{
		"repository": {
			provider: fmt.Sprintf("repository.New%sRepository", camel),
			pattern:  `(RepositorySet = wire\.NewSet\([\s\S]*?\n)(\s*\))`,
			insert:   fmt.Sprintf("${1}\t\trepository.New%sRepository,\n${2}", camel),
		},
		"service": {
			provider: fmt.Sprintf("service.New%sService", camel),
			pattern:  `(ServiceSet = wire\.NewSet\([\s\S]*?\n)(\s*\))`,
			insert:   fmt.Sprintf("${1}\t\tservice.New%sService,\n${2}", camel),
		},
		"handler": {
			provider: fmt.Sprintf("handler.New%sHandler", camel),
			pattern:  `(HandlerSet = wire\.NewSet\([\s\S]*?\n)(\s*\))`,
			insert:   fmt.Sprintf("${1}\t\thandler.New%sHandler,\n${2}", camel),
		},
	}

	for _, replacement := range replacements {
		// 已存在则跳过，防止重复追加
		if strings.Contains(text, replacement.provider) {
			continue
		}
		re := regexp.MustCompile(replacement.pattern)
		newText := re.ReplaceAllString(text, replacement.insert)
		if newText != text {
			text = newText
		}
	}

	_ = lcamel // reserved for future use

	if text != string(content) {
		return os.WriteFile(wirePath, []byte(text), 0644)
	}
	return nil
}
