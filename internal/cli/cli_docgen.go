package cli

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wonli/aqi/internal/docgen"
	"gopkg.in/yaml.v3"
)

//go:embed templates/docgen/*
var docgenTemplateFS embed.FS

var (
	docgenRouterDirFlag   string
	docgenConfigFileFlag  string
	docgenPackageNameFlag string
	docgenFormatFlag      string
)

func randomHex(bytesLen int) (string, error) {
	if bytesLen <= 0 {
		bytesLen = 16
	}
	b := make([]byte, bytesLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ensureAuthSection ensures doc-config.yaml contains auth section and generates random password when missing/empty.
// Only mutates when auth is missing or password is empty/default.
func ensureAuthSection(configPath string) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return
	}

	// Use a loose map to avoid coupling to docgen structs.
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		return
	}

	authAny, ok := m["auth"]
	authMap, _ := authAny.(map[string]any)
	if !ok || authMap == nil {
		authMap = map[string]any{}
	}

	// Defaults
	if _, exists := authMap["enabled"]; !exists {
		authMap["enabled"] = false
	}
	if _, exists := authMap["username"]; !exists {
		authMap["username"] = "admin"
	}
	if _, exists := authMap["realm"]; !exists {
		authMap["realm"] = "API 文档"
	}

	pwd, _ := authMap["password"].(string)
	if pwd == "" || pwd == "admin" {
		if rp, err := randomHex(16); err == nil {
			authMap["password"] = rp
		} else {
			authMap["password"] = "change_me"
		}
	}

	m["auth"] = authMap

	out, err := yaml.Marshal(m)
	if err != nil {
		return
	}
	_ = os.WriteFile(configPath, out, 0644)
}

var docgenCmd = &cobra.Command{
	Use:   "docgen",
	Short: "Documentation generation tools",
}

var docgenInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize documentation generator in project",
	Run: func(cmd *cobra.Command, args []string) {
		// 获取当前工作目录
		workDir, err := os.Getwd()
		if err != nil {
			fmt.Printf("Error getting working directory: %v\n", err)
			os.Exit(1)
		}

		// 确定包名
		packageName := docgenPackageNameFlag
		if packageName == "" {
			// 尝试从go.mod获取包名
			packageName = getPackageName(workDir)
		}
		if packageName == "" {
			packageName = "main"
		}

		// 创建docs目录
		docsDir := filepath.Join(workDir, "docs")
		if err := os.MkdirAll(docsDir, 0755); err != nil {
			fmt.Printf("Error creating docs directory: %v\n", err)
			os.Exit(1)
		}

		// 生成docgen.go文件
		if err := generateDocgenFile(docsDir, packageName, workDir); err != nil {
			fmt.Printf("Error generating docgen.go: %v\n", err)
			os.Exit(1)
		}

		format := strings.ToLower(docgenFormatFlag)
		if format != "json" && format != "markdown" {
			fmt.Printf("警告: 格式 '%s' 无效，使用默认格式 'json'\n", docgenFormatFlag)
			format = "json"
		}
		fmt.Printf("Successfully generated docs/docgen.go (format: %s)\n", format)
	},
}

func init() {
	docgenInitCmd.Flags().StringVarP(&docgenRouterDirFlag, "router", "r", "./internal/router", "路由文件目录")
	docgenInitCmd.Flags().StringVarP(&docgenConfigFileFlag, "config", "c", "", "文档配置文件路径（可选）")
	docgenInitCmd.Flags().StringVarP(&docgenPackageNameFlag, "package", "p", "", "包名（可选，默认从go.mod获取）")
	docgenInitCmd.Flags().StringVarP(&docgenFormatFlag, "format", "f", "json", "输出格式：json 或 markdown（默认: json）")
	docgenCmd.AddCommand(docgenInitCmd)
	rootCmd.AddCommand(docgenCmd)
}

// getPackageName 从go.mod获取包名
func getPackageName(workDir string) string {
	goModPath := filepath.Join(workDir, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimPrefix(line, "module ")
		}
	}
	return ""
}

// generateDocgenFile 生成docgen.go文件
func generateDocgenFile(docsDir, packageName, workDir string) error {
	// 1. 运行docgen生成JSON文件到docs目录（会同时生成配置文件）
	routerDir := docgenRouterDirFlag
	if !filepath.IsAbs(routerDir) {
		routerDir = filepath.Join(workDir, routerDir)
	}

	// 检查router目录是否存在
	if _, err := os.Stat(routerDir); os.IsNotExist(err) {
		fmt.Printf("警告: 路由目录不存在: %s，将跳过JSON文件生成\n", routerDir)
		fmt.Printf("请稍后运行 'aqi docgen init' 重新生成\n")
	} else {
		// 直接调用docgen功能生成文档文件（会生成配置文件到docs目录）
		format := strings.ToLower(docgenFormatFlag)
		if format != "json" && format != "markdown" {
			format = "json" // 默认使用 json
		}
		if err := runDocgen(routerDir, docsDir, docgenConfigFileFlag, workDir, format); err != nil {
			fmt.Printf("警告: 生成文档文件失败: %v\n", err)
			fmt.Printf("请检查路由目录和配置文件是否正确\n")
		} else {
			formatName := "JSON"
			if format == "markdown" {
				formatName = "Markdown"
			}
			fmt.Printf("已生成%s文档文件到 docs 目录\n", formatName)
		}
	}

	// 2. 确保配置文件在docs目录（用于embed）
	// 如果配置文件不存在，从embed的模板复制
	configFileInDocs := filepath.Join(docsDir, "doc-config.yaml")
	if _, err := os.Stat(configFileInDocs); os.IsNotExist(err) {
		// 从embed的模板读取默认配置
		defaultConfig, err := docgenTemplateFS.ReadFile("templates/docgen/doc-config.yaml")
		if err == nil {
			if err := os.WriteFile(configFileInDocs, defaultConfig, 0644); err == nil {
				fmt.Printf("已从模板创建配置文件: %s\n", configFileInDocs)
				// 首次创建时确保 auth 段存在，并写入随机密码
				ensureAuthSection(configFileInDocs)
			}
		} else {
			fmt.Printf("警告: 无法读取默认配置模板: %v\n", err)
		}
	}

	// 3. 读取配置文件获取文档列表并注入到HTML
	htmlTemplateContent, err := docgenTemplateFS.ReadFile("templates/docgen/api_viewer.html")
	if err != nil {
		return fmt.Errorf("failed to read HTML template: %w", err)
	}

	var docsConfigJSON string = "[]"
	if configData, err := os.ReadFile(configFileInDocs); err == nil {
		var config struct {
			AppDocuments []struct {
				Name  string `yaml:"name" json:"name"`
				Label string `yaml:"label" json:"label"`
				File  string `yaml:"file" json:"file"`
			} `yaml:"appDocuments" json:"appDocuments"`
		}
		if err := yaml.Unmarshal(configData, &config); err == nil {
			if jsonData, err := json.Marshal(config.AppDocuments); err == nil {
				docsConfigJSON = string(jsonData)
			}
		}
	}

	// 直接使用字符串替换注入文档列表（避免与Vue.js的{{ }}语法冲突）
	// 将 {{.DocsConfig}} 替换为实际的JSON字符串
	htmlContent := strings.ReplaceAll(string(htmlTemplateContent), "{{.DocsConfig}}", docsConfigJSON)

	// 写入HTML文件
	htmlPath := filepath.Join(docsDir, "api_viewer.html")
	if err := os.WriteFile(htmlPath, []byte(htmlContent), 0644); err != nil {
		return fmt.Errorf("failed to write HTML file: %w", err)
	}
	fmt.Printf("已生成 api_viewer.html 到 docs 目录（已注入文档列表）\n")

	// 4. 读取docgen.go模板
	tmplContent, err := docgenTemplateFS.ReadFile("templates/docgen/docgen.go.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read template: %w", err)
	}

	// 5. 根据格式确定 embed 模式
	format := strings.ToLower(docgenFormatFlag)
	if format != "json" && format != "markdown" {
		format = "json" // 默认使用 json
	}
	var embedPattern string
	if format == "markdown" {
		embedPattern = "api_viewer.html *.md doc-config.yaml"
	} else {
		embedPattern = "api_viewer.html *.json doc-config.yaml"
	}

	// 替换模板中的 embed 指令
	tmplContentStr := strings.ReplaceAll(string(tmplContent), "api_viewer.html *.json doc-config.yaml", embedPattern)

	// 6. 解析模板
	tmpl, err := template.New("docgen.go").Parse(tmplContentStr)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	// 7. 准备数据
	tmplData := struct {
		PackageName string
	}{
		PackageName: packageName,
	}

	// 8. 生成文件
	outputPath := filepath.Join(docsDir, "docgen.go")
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outputFile.Close()

	if err := tmpl.Execute(outputFile, tmplData); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

// runDocgen 直接调用docgen功能生成JSON文件
func runDocgen(routerDir, outputDir, configFile, workDir, format string) error {
	// 1. 扫描路由文件
	routerFiles, err := docgen.ScanRouterFiles(routerDir)
	if err != nil {
		return fmt.Errorf("扫描路由文件失败: %w", err)
	}

	if len(routerFiles) == 0 {
		return fmt.Errorf("未找到路由文件")
	}

	// 2. 先快速解析所有 action 名称（用于生成默认配置）
	var tempActions []docgen.ActionDoc
	for i := range routerFiles {
		rf := &routerFiles[i]
		actions, err := docgen.ParseRouterFile(filepath.Join(routerDir, rf.FileName), rf.FuncName)
		if err != nil {
			// 如果解析失败，继续处理其他文件
			continue
		}
		tempActions = append(tempActions, actions...)
	}

	// 3. 确定配置文件路径（生成到docs目录）
	actualConfigPath := configFile
	if actualConfigPath == "" {
		// 默认生成到 docs 目录
		actualConfigPath = filepath.Join(outputDir, "doc-config.yaml")
	} else if !filepath.IsAbs(actualConfigPath) {
		actualConfigPath = filepath.Join(workDir, actualConfigPath)
	}

	// 4. 加载分类配置（如果配置文件不存在，会根据 actions 自动生成）
	if err := docgen.LoadCategoryConfig(actualConfigPath, tempActions); err != nil {
		return fmt.Errorf("加载配置文件失败: %w", err)
	}
	fmt.Printf("已加载分类配置文件: %s\n", actualConfigPath)

	// 5. 重新解析所有路由文件（这次会使用配置进行分类）
	var allActions []docgen.ActionDoc
	for i := range routerFiles {
		rf := &routerFiles[i]
		fmt.Printf("解析路由文件: %s (函数: %s)\n", rf.FileName, rf.FuncName)
		actions, err := docgen.ParseRouterFile(filepath.Join(routerDir, rf.FileName), rf.FuncName)
		if err != nil {
			fmt.Printf("解析路由文件 %s 失败: %v\n", rf.FileName, err)
			continue
		}
		rf.Actions = actions
		fmt.Printf("  找到 %d 个 action\n", len(actions))
		allActions = append(allActions, actions...)
	}

	fmt.Printf("总共解析 %d 个 action\n", len(allActions))

	// 6. 基于所有路由文件生成全局接口更新日志
	var globalChangelog *docgen.ChangelogEntry
	if format == "json" {
		changelog, err := docgen.GenerateGlobalChangelog(outputDir, routerFiles)
		if err != nil {
			fmt.Printf("警告: 生成全局接口更新日志失败: %v\n", err)
		} else if changelog != nil {
			globalChangelog = changelog
			fmt.Printf("已生成全局接口更新日志（版本: %s, 新增: %d, 删除: %d）\n",
				changelog.Version, len(changelog.Added), len(changelog.Removed))
		}
	}

	// 7. 为每个路由文件生成独立的文档，并收集文档信息
	var documents []docgen.DocumentInfo
	for _, rf := range routerFiles {
		if len(rf.Actions) == 0 {
			continue
		}

		// 生成文件名
		baseName := strings.TrimSuffix(rf.FileName, ".go")
		var docFileName string
		var outputPath string
		if format == "markdown" {
			docFileName = fmt.Sprintf("cmd_api_%s.md", baseName)
			outputPath = filepath.Join(outputDir, docFileName)
			// 为单个路由文件生成 Markdown 文档
			singleRouterFiles := []docgen.RouterFile{rf}
			if err := docgen.GenerateMarkdown(singleRouterFiles, outputPath); err != nil {
				fmt.Printf("生成 Markdown 文档失败 (%s): %v\n", rf.FileName, err)
				continue
			}
		} else {
			docFileName = fmt.Sprintf("cmd_api_%s.json", baseName)
			outputPath = filepath.Join(outputDir, docFileName)
			// 为单个路由文件生成 JSON 文档，传入全局 changelog
			singleRouterFiles := []docgen.RouterFile{rf}
			if err := docgen.GenerateJSON(singleRouterFiles, outputPath, globalChangelog); err != nil {
				fmt.Printf("生成 JSON 文档失败 (%s): %v\n", rf.FileName, err)
				continue
			}
		}

		// 生成文档名称和标签
		docName := baseName
		// 将 baseName 转换为标题格式（例如 "action_admin" -> "Action Admin API"）
		parts := strings.Split(strings.ReplaceAll(baseName, "_", " "), " ")
		for i, part := range parts {
			if len(part) > 0 {
				parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
			}
		}
		docLabel := fmt.Sprintf("%s API", strings.Join(parts, " "))

		// 添加到文档列表
		documents = append(documents, docgen.DocumentInfo{
			Name:  docName,
			Label: docLabel,
			File:  docFileName,
		})

		fmt.Printf("文档生成成功: %s (%d 个 action)\n", outputPath, len(rf.Actions))
	}

	// 8. 更新配置文件中的文档列表
	if len(documents) > 0 {
		if err := docgen.UpdateDocumentsInConfig(actualConfigPath, documents, format); err != nil {
			fmt.Printf("警告: 更新配置文件中的文档列表失败: %v\n", err)
		} else {
			fmt.Printf("已更新配置文件中的文档列表\n")
		}
	}

	return nil
}
