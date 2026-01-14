package docgen

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ScanRouterFiles 扫描路由文件
func ScanRouterFiles(routerDir string) ([]RouterFile, error) {
	var routerFiles []RouterFile

	err := filepath.Walk(routerDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 扫描所有 .go 文件，查找包含 Add 调用的路由注册函数
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".go") {
			// 解析文件，查找路由注册函数
			fset := token.NewFileSet()
			node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
			if err != nil {
				return fmt.Errorf("解析文件 %s 失败: %w", path, err)
			}

			// 查找包含 Add 调用的函数
			ast.Inspect(node, func(n ast.Node) bool {
				fn, ok := n.(*ast.FuncDecl)
				if !ok {
					return true
				}

				// 检查函数体中是否有 Add 调用
				if fn.Body != nil {
					hasAdd := false
					ast.Inspect(fn.Body, func(n ast.Node) bool {
						call, ok := n.(*ast.CallExpr)
						if !ok {
							return true
						}
						if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
							if sel.Sel.Name == "Add" {
								hasAdd = true
								return false
							}
						}
						return true
					})

					if hasAdd {
						// 跳过辅助函数（通过函数名判断，可以配置）
						// 这里可以根据需要扩展跳过规则
						skipFuncs := map[string]bool{
							"events": true,
						}
						if skipFuncs[fn.Name.Name] {
							return true
						}
						routerFiles = append(routerFiles, RouterFile{
							FileName: filepath.Base(path),
							FuncName: fn.Name.Name,
						})
						return false
					}
				}
				return true
			})
		}
		return nil
	})

	return routerFiles, err
}

// ParseRouterFile 解析路由文件
func ParseRouterFile(filePath, funcName string) ([]ActionDoc, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("解析文件失败: %w", err)
	}

	// 查找路由注册函数
	var routerFunc *ast.FuncDecl
	ast.Inspect(node, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == funcName {
			routerFunc = fn
			return false
		}
		return true
	})

	if routerFunc == nil {
		return nil, fmt.Errorf("未找到函数 %s", funcName)
	}

	// 构建中间件链映射
	middlewareMap := buildMiddlewareMap(routerFunc, node)

	// 解析所有 Add 调用
	var actions []ActionDoc
	ast.Inspect(routerFunc.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Add" {
			return true
		}

		// 提取 action 名称和 handler
		if len(call.Args) < 2 {
			return true
		}

		// 第一个参数：action 名称
		actionName := extractStringLiteral(call.Args[0])
		if actionName == "" {
			return true
		}

		// 第二个参数：handler 函数
		handler := call.Args[1]

		// 获取中间件组变量名
		var middlewareGroup string
		if ident, ok := sel.X.(*ast.Ident); ok {
			middlewareGroup = ident.Name
		}

		// 获取中间件链
		middlewareChain := middlewareMap[middlewareGroup]

		action := ActionDoc{
			Name:            actionName,
			RouterFile:      filepath.Base(filePath),
			MiddlewareGroup: middlewareGroup,
			MiddlewareChain: middlewareChain,
			Category:        inferCategory(actionName),
			Params:          []ParamField{},
			Returns:         ReturnType{ErrorCodes: []ErrorCode{}},
		}

		// 解析 handler 函数（传入文件路径以便查找外部包）
		if err := parseHandler(handler, &action, node, fset, filePath); err != nil {
			fmt.Printf("警告: 解析 handler 失败 (%s): %v\n", actionName, err)
		}

		actions = append(actions, action)
		return true
	})

	return actions, nil
}

// buildMiddlewareMap 构建中间件链映射
func buildMiddlewareMap(routerFunc *ast.FuncDecl, file *ast.File) map[string][]string {
	middlewareMap := make(map[string][]string)

	// 第一遍：收集所有变量声明和 Use 调用
	type varInfo struct {
		varName     string
		parentVar   string
		middlewares []string
		isNewRouter bool
	}

	var varInfos []varInfo

	ast.Inspect(routerFunc.Body, func(n ast.Node) bool {
		if assign, ok := n.(*ast.AssignStmt); ok && len(assign.Lhs) > 0 && len(assign.Rhs) > 0 {
			if ident, ok := assign.Lhs[0].(*ast.Ident); ok {
				varName := ident.Name
				info := varInfo{varName: varName}

				// 检查是否是 ws.NewRouter()
				if call, ok := assign.Rhs[0].(*ast.CallExpr); ok {
					if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
						if sel.Sel.Name == "NewRouter" {
							info.isNewRouter = true
							info.middlewares = []string{}
						} else if sel.Sel.Name == "Use" {
							// 获取父变量
							if parentIdent, ok := sel.X.(*ast.Ident); ok {
								info.parentVar = parentIdent.Name
							}
							// 提取中间件
							info.middlewares = extractMiddlewares(call.Args)
						}
					}
				}
				varInfos = append(varInfos, info)
			}
		}
		return true
	})

	// 第二遍：构建完整的中间件链（处理继承关系）
	for _, info := range varInfos {
		if info.isNewRouter {
			middlewareMap[info.varName] = []string{}
		} else if info.parentVar != "" {
			// 继承父变量的中间件链
			parentChain := middlewareMap[info.parentVar]
			fullChain := make([]string, len(parentChain))
			copy(fullChain, parentChain)
			fullChain = append(fullChain, info.middlewares...)
			middlewareMap[info.varName] = fullChain
		} else {
			middlewareMap[info.varName] = info.middlewares
		}
	}

	return middlewareMap
}

// extractMiddlewares 提取中间件列表
func extractMiddlewares(args []ast.Expr) []string {
	var middlewares []string
	for _, arg := range args {
		// 处理多个中间件参数
		if call, ok := arg.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				middlewareName := sel.Sel.Name
				middlewares = append(middlewares, middlewareName)
			}
		}
	}
	return middlewares
}

// extractStringLiteral 提取字符串字面量
func extractStringLiteral(expr ast.Expr) string {
	if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		return strings.Trim(lit.Value, `"`)
	}
	return ""
}

// metadataConfig 元数据配置
type metadataConfig struct {
	// ProjectName 项目名称
	ProjectName string `yaml:"projectName" json:"projectName"`
	// Version 版本号
	Version string `yaml:"version" json:"version"`
	// Description 项目描述
	Description string `yaml:"description" json:"description"`
	// Author 作者
	Author string `yaml:"author" json:"author"`
	// Contact 联系方式
	Contact string `yaml:"contact" json:"contact"`
}

// middlewareGroupConfig 中间件组名称配置
type middlewareGroupConfig struct {
	// MiddlewareMap 统一的中间件映射配置（同时支持中间件链和中间件组变量名作为 key）
	// 中间件链格式：使用下划线分隔的小写格式，如 recovery_app_auth -> "需要登录"（无需引号）
	// 匹配优先级：中间件链 > 中间件组变量名
	MiddlewareMap map[string]string `yaml:"middlewareMap" json:"middlewareMap"`
	// DefaultTemplate 默认组名称模板（支持 %s 占位符，会被中间件名称列表替换）
	DefaultTemplate string `yaml:"defaultTemplate" json:"defaultTemplate"`
	// NoMiddleware 无中间件的组名称
	NoMiddleware string `yaml:"noMiddleware" json:"noMiddleware"`
}

// DocumentInfo 文档信息
type DocumentInfo struct {
	Name  string `yaml:"name" json:"name"`   // 文档名称（如 "action", "action-admin"）
	Label string `yaml:"label" json:"label"` // 显示标签（如 "Action API", "Action Admin API"）
	File  string `yaml:"file" json:"file"`   // JSON 文件名（如 "cmd_api_action.json"）
}

// authConfig 文档访问身份认证配置
type authConfig struct {
	Enabled  bool   `yaml:"enabled" json:"enabled"`
	Username string `yaml:"username" json:"username"`
	Password string `yaml:"password" json:"password"`
	Realm    string `yaml:"realm" json:"realm"`
}

// categoryConfig 分类配置
type categoryConfig struct {
	// Metadata 元数据配置
	Metadata metadataConfig `yaml:"metadata" json:"metadata"`
	// CategoryMap 分类映射（完全匹配优先级最高，否则使用点号前缀匹配）
	CategoryMap map[string]string `yaml:"categoryMap" json:"categoryMap"`
	// MiddlewareGroup 中间件组名称配置
	MiddlewareGroup middlewareGroupConfig `yaml:"middlewareGroup" json:"middlewareGroup"`
	// AppDocuments 文档列表（生成的 JSON 文档文件信息）
	AppDocuments []DocumentInfo `yaml:"appDocuments" json:"appDocuments"`
	// Auth 身份认证配置（用于文档访问保护）
	Auth authConfig `yaml:"auth" json:"auth"`
}

// globalCategoryConfig 全局分类配置
var globalCategoryConfig *categoryConfig

// getGitCommitCount 获取 git commit 计数（用于版本追踪）
func getGitCommitCount() string {
	// 检查是否在 git 仓库中
	if _, err := os.Stat(".git"); os.IsNotExist(err) {
		return "-"
	}

	// 检查 git 命令是否可用
	if _, err := exec.LookPath("git"); err != nil {
		return "-"
	}

	// 获取 commit 计数
	commitCmd := exec.Command("git", "rev-list", "--count", "HEAD")
	commitOutput, err := commitCmd.Output()
	if err != nil {
		return "-"
	}
	commit := strings.TrimSpace(string(commitOutput))
	if commit == "" {
		return "-"
	}

	return commit
}

// getGitVersion 获取 git 版本信息（分支+commit计数+revision）
// 格式：branch-commit-revision，如果不在 git 仓库中、git 未安装或命令失败，返回 "-"
func getGitVersion() string {
	// 检查是否在 git 仓库中
	if _, err := os.Stat(".git"); os.IsNotExist(err) {
		return "-"
	}

	// 检查 git 命令是否可用
	if _, err := exec.LookPath("git"); err != nil {
		return "-"
	}

	// 获取分支名
	branchCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	branchOutput, err := branchCmd.Output()
	if err != nil {
		return "-"
	}
	branch := strings.TrimSpace(string(branchOutput))
	if branch == "" {
		return "-"
	}

	// 获取 commit 计数
	commitCmd := exec.Command("git", "rev-list", "--count", "HEAD")
	commitOutput, err := commitCmd.Output()
	if err != nil {
		return "-"
	}
	commit := strings.TrimSpace(string(commitOutput))
	if commit == "" {
		return "-"
	}

	// 获取 revision（短 commit hash）
	revisionCmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	revisionOutput, err := revisionCmd.Output()
	if err != nil {
		return "-"
	}
	revision := strings.TrimSpace(string(revisionOutput))
	if revision == "" {
		return "-"
	}

	return fmt.Sprintf("%s-%s-%s", branch, commit, revision)
}

func generateRandomPasswordHex(bytesLen int) (string, error) {
	if bytesLen <= 0 {
		bytesLen = 16
	}
	b := make([]byte, bytesLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// generateDefaultConfig 根据 action 列表生成默认配置文件
func generateDefaultConfig(configPath string, actions []ActionDoc) error {
	// 只收集前缀（第一个点号之前的部分）
	categoryMap := make(map[string]string)
	prefixSet := make(map[string]bool)

	// 统一的中间件映射
	middlewareMap := make(map[string]string)

	for _, action := range actions {
		// 提取前缀（第一个点号之前的部分）
		parts := strings.Split(action.Name, ".")
		if len(parts) > 0 {
			prefix := parts[0]
			if !prefixSet[prefix] {
				prefixSet[prefix] = true
				// 添加前缀（值=键）
				categoryMap[prefix] = prefix
			}
		} else {
			// 如果没有点号，使用完整名称
			if !prefixSet[action.Name] {
				prefixSet[action.Name] = true
				categoryMap[action.Name] = action.Name
			}
		}

		// 只收集中间件链（使用下划线分隔的小写格式，更简洁，不需要引号）
		// 不收集单个中间件组变量名，只保留中间件链配置
		if len(action.MiddlewareChain) > 0 {
			chainKey := strings.ToLower(strings.Join(action.MiddlewareChain, "_"))
			if _, exists := middlewareMap[chainKey]; !exists {
				middlewareMap[chainKey] = chainKey
			}
		}
	}

	// 创建默认配置对象（用于中间件配置的默认值与可扩展性）
	randomPwd, err := generateRandomPasswordHex(16)
	if err != nil {
		// 随机失败时回退到一个固定但明显的默认值，避免生成空密码
		randomPwd = "change_me"
	}
	// 获取 git 版本信息
	gitVersion := getGitVersion()

	config := categoryConfig{
		Metadata: metadataConfig{
			ProjectName: "API 文档",
			Version:     gitVersion,
			Description: "WebSocket API 文档",
		},
		CategoryMap: categoryMap,
		MiddlewareGroup: middlewareGroupConfig{
			MiddlewareMap:   middlewareMap,
			DefaultTemplate: "%s",
			NoMiddleware:    "默认",
		},
		Auth: authConfig{
			Enabled:  false,
			Username: "admin",
			Password: randomPwd,
			Realm:    "API 文档",
		},
	}

	// 手动生成 YAML 格式，确保格式统一
	var buf strings.Builder

	// 添加注释头部
	buf.WriteString("# 文档配置文件\n")
	buf.WriteString("# 用于定义 WebSocket Action 的分类规则和项目元数据\n")
	buf.WriteString("# \n")
	buf.WriteString("# 注意：categoryMap 中的值已自动设置为键值，请根据需要修改为合适的中文分类名称\n\n")

	// metadata
	buf.WriteString("metadata:\n")
	buf.WriteString("    projectName: \"API 文档\"\n")
	buf.WriteString(fmt.Sprintf("    version: \"%s\"\n", gitVersion))
	buf.WriteString("    description: \"WebSocket API 文档\"\n")
	buf.WriteString("    author: \"\"\n")
	buf.WriteString("    contact: \"\"\n\n")

	// categoryMap
	buf.WriteString("categoryMap:\n")
	// 按键排序
	keys := make([]string, 0, len(categoryMap))
	for k := range categoryMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		buf.WriteString(fmt.Sprintf("    %s: \"%s\"\n", k, categoryMap[k]))
	}

	// middlewareGroup
	buf.WriteString("\nmiddlewareGroup:\n")

	// middlewareMap（统一的中间件映射，只包含中间件链，不包含单个中间件组变量名）
	buf.WriteString("    # 中间件链映射配置（使用下划线分隔的小写格式，如 recovery_app_auth -> \"需要登录\"）\n")
	buf.WriteString("    middlewareMap:\n")
	if len(config.MiddlewareGroup.MiddlewareMap) > 0 {
		mKeys := make([]string, 0, len(config.MiddlewareGroup.MiddlewareMap))
		for k := range config.MiddlewareGroup.MiddlewareMap {
			mKeys = append(mKeys, k)
		}
		sort.Strings(mKeys)
		for _, k := range mKeys {
			// 只输出中间件链（包含下划线的），跳过单个中间件组变量名
			if strings.Contains(k, "_") {
				buf.WriteString(fmt.Sprintf("        %s: \"%s\"\n", k, config.MiddlewareGroup.MiddlewareMap[k]))
			}
		}
	} else {
		buf.WriteString("        # 暂无配置\n")
	}
	buf.WriteString("\n")

	// 默认模板与无中间件文案
	buf.WriteString(fmt.Sprintf("    defaultTemplate: \"%s\"\n", config.MiddlewareGroup.DefaultTemplate))
	buf.WriteString("\n")
	buf.WriteString(fmt.Sprintf("    noMiddleware: \"%s\"\n", config.MiddlewareGroup.NoMiddleware))

	// appDocuments 文档列表（初始为空，会在生成文档后更新）
	buf.WriteString("\nappDocuments: []\n")

	// auth 身份认证配置（默认关闭，但生成随机密码，提升安全性）
	buf.WriteString("\nauth:\n")
	buf.WriteString(fmt.Sprintf("    enabled: %v\n", config.Auth.Enabled))
	buf.WriteString(fmt.Sprintf("    username: \"%s\"\n", config.Auth.Username))
	buf.WriteString(fmt.Sprintf("    password: \"%s\"\n", config.Auth.Password))
	buf.WriteString(fmt.Sprintf("    realm: \"%s\"\n", config.Auth.Realm))

	fullData := []byte(buf.String())

	// 写入文件
	if err := os.WriteFile(configPath, fullData, 0644); err != nil {
		return fmt.Errorf("写入默认配置文件失败: %w", err)
	}

	return nil
}

// loadCategoryConfig 从配置文件加载分类配置
// 支持 YAML 和 JSON 格式，如果配置文件不存在，会根据 actions 自动生成默认配置
func LoadCategoryConfig(configPath string, actions []ActionDoc) error {
	// 如果未指定配置文件，使用默认位置的 YAML 文件
	if configPath == "" {
		configPath = "./cmd/docgen/doc-config.yaml"
	}

	// 检查配置文件是否存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// 配置文件不存在，生成默认配置
		fmt.Printf("配置文件不存在，正在生成默认配置文件: %s\n", configPath)
		if err := generateDefaultConfig(configPath, actions); err != nil {
			return fmt.Errorf("生成默认配置文件失败: %w", err)
		}
		fmt.Printf("已生成默认配置文件，请根据需要修改 categoryMap 的值\n")
	}

	// 读取配置文件
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	var config categoryConfig
	// 根据文件扩展名决定解析方式
	if strings.HasSuffix(configPath, ".yaml") || strings.HasSuffix(configPath, ".yml") {
		if err := yaml.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("解析 YAML 配置文件失败: %w", err)
		}
	} else {
		// 默认尝试 JSON
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("解析 JSON 配置文件失败: %w", err)
		}
	}

	// 验证必需字段
	if config.CategoryMap == nil {
		config.CategoryMap = make(map[string]string)
	}
	if config.Metadata.ProjectName == "" {
		config.Metadata.ProjectName = "API 文档"
	}
	// 设置默认中间件组配置
	if config.MiddlewareGroup.MiddlewareMap == nil {
		config.MiddlewareGroup.MiddlewareMap = make(map[string]string)
	}
	if config.MiddlewareGroup.DefaultTemplate == "" {
		config.MiddlewareGroup.DefaultTemplate = "%s"
	}
	if config.MiddlewareGroup.NoMiddleware == "" {
		config.MiddlewareGroup.NoMiddleware = "默认"
	}

	globalCategoryConfig = &config
	return nil
}

// UpdateDocumentsInConfig 更新配置文件中的文档列表
// 会合并自动生成的文档和用户手动添加的文档，避免覆盖用户自定义设置
// format 参数指定当前生成的格式（"json" 或 "markdown"），用于过滤掉不匹配格式的文档
func UpdateDocumentsInConfig(configPath string, documents []DocumentInfo, format string) error {
	// 读取现有配置
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	var config categoryConfig
	if strings.HasSuffix(configPath, ".yaml") || strings.HasSuffix(configPath, ".yml") {
		if err := yaml.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("解析 YAML 配置文件失败: %w", err)
		}
	} else {
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("解析 JSON 配置文件失败: %w", err)
		}
	}

	// 创建自动生成文档的映射表（以 file 为 key）
	autoGenMap := make(map[string]DocumentInfo)
	for _, doc := range documents {
		if doc.File != "" {
			autoGenMap[doc.File] = doc
		}
	}

	// 确定当前格式的文件扩展名
	var currentExt string
	if format == "markdown" {
		currentExt = ".md"
	} else {
		currentExt = ".json" // 默认 json
	}

	// 合并策略：
	// 1. 对于现有配置中的文档，如果 file 在自动生成列表中，则更新（保留用户的 name 和 label）
	// 2. 如果 file 不在自动生成列表中，但格式匹配当前格式，则保留（这是用户手动添加的同格式文档）
	// 3. 如果 file 格式不匹配当前格式，则移除（避免混合格式）
	// 4. 对于自动生成的文档，如果 file 不在现有配置中，则添加
	var mergedDocs []DocumentInfo
	existingFileMap := make(map[string]bool)

	// 先处理现有配置中的文档
	for _, existingDoc := range config.AppDocuments {
		// 检查文件扩展名是否匹配当前格式
		hasCurrentExt := strings.HasSuffix(existingDoc.File, currentExt)

		if _, exists := autoGenMap[existingDoc.File]; exists {
			// 文件存在，更新但保留用户的 name 和 label（如果用户有自定义）
			mergedDocs = append(mergedDocs, DocumentInfo{
				Name:  existingDoc.Name,  // 保留用户自定义的 name
				Label: existingDoc.Label, // 保留用户自定义的 label
				File:  existingDoc.File,  // 使用现有的 file（应该和自动生成的一致）
			})
			existingFileMap[existingDoc.File] = true
		} else if existingDoc.File != "" && hasCurrentExt {
			// 文件不在自动生成列表中，但格式匹配，保留用户手动添加的同格式文档
			mergedDocs = append(mergedDocs, existingDoc)
			existingFileMap[existingDoc.File] = true
		}
	}

	// 添加自动生成但不在现有配置中的文档
	for _, autoGenDoc := range documents {
		if !existingFileMap[autoGenDoc.File] {
			mergedDocs = append(mergedDocs, autoGenDoc)
		}
	}

	// 更新文档列表
	config.AppDocuments = mergedDocs

	// 写回配置文件
	var outputData []byte
	if strings.HasSuffix(configPath, ".yaml") || strings.HasSuffix(configPath, ".yml") {
		outputData, err = yaml.Marshal(&config)
		if err != nil {
			return fmt.Errorf("序列化 YAML 配置失败: %w", err)
		}
	} else {
		outputData, err = json.MarshalIndent(&config, "", "    ")
		if err != nil {
			return fmt.Errorf("序列化 JSON 配置失败: %w", err)
		}
	}

	if err := os.WriteFile(configPath, outputData, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	return nil
}

// GetDocuments 获取配置中的文档列表
func GetDocuments() []DocumentInfo {
	if globalCategoryConfig == nil {
		return nil
	}
	return globalCategoryConfig.AppDocuments
}

// inferCategory 根据 action 名称推断分类
// 注意：这是项目特定的业务逻辑，分类映射通过配置文件定义
// 匹配规则：完全匹配优先级最高，否则使用点号前缀匹配，最后使用前缀作为分类名称
func inferCategory(actionName string) string {
	// 配置必须已加载，如果为 nil 说明配置加载失败
	if globalCategoryConfig == nil {
		// 如果配置未加载，尝试使用前缀
		parts := strings.Split(actionName, ".")
		if len(parts) > 0 {
			return parts[0]
		}
		return actionName
	}

	// 首先尝试完全匹配（优先级最高）
	if category, ok := globalCategoryConfig.CategoryMap[actionName]; ok {
		return category
	}

	// 然后尝试前缀匹配（使用点号分割）
	parts := strings.Split(actionName, ".")
	if len(parts) > 0 {
		prefix := parts[0]
		if category, ok := globalCategoryConfig.CategoryMap[prefix]; ok {
			return category
		}
		// 如果前缀匹配失败，直接使用前缀作为分类名称
		return prefix
	}

	// 如果没有点号，直接使用 action 名称作为分类
	return actionName
}

// getMetadata 获取元数据配置
func getMetadata() metadataConfig {
	if globalCategoryConfig == nil {
		// 返回空元数据
		return metadataConfig{
			ProjectName: "API 文档",
		}
	}
	return globalCategoryConfig.Metadata
}

// parseHandler 解析 handler 函数
func parseHandler(handler ast.Expr, action *ActionDoc, file *ast.File, fset *token.FileSet, routerFilePath string) error {
	// 处理 SelectorExpr: login.ActionSms
	if sel, ok := handler.(*ast.SelectorExpr); ok {
		// 获取包名和函数名
		var pkgName, funcName string
		if pkgIdent, ok := sel.X.(*ast.Ident); ok {
			pkgName = pkgIdent.Name
			funcName = sel.Sel.Name
		}

		if pkgName != "" && funcName != "" {
			// 从 import 语句中找到包的完整路径
			packagePath := findPackagePath(pkgName, file)
			if packagePath != "" {
				handlerFile := findHandlerFile(packagePath, funcName)
				if handlerFile != "" {
					parseExternalHandler(handlerFile, funcName, action)
				}
			}
		}
		return nil
	}

	// 如果是函数标识符，查找函数定义
	if ident, ok := handler.(*ast.Ident); ok {
		// 在当前文件中查找函数定义
		found := false
		ast.Inspect(file, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == ident.Name {
				// 获取当前文件所在目录，用于查找同一包内的类型
				packageDir := ""
				if routerFilePath != "" {
					packageDir = filepath.Dir(routerFilePath)
				}
				// 如果无法从文件路径获取，尝试从文件内容推断
				if packageDir == "" {
					// 尝试从 import 路径推断包目录（作为后备方案）
					// 这里暂时使用空字符串，让 extractStructFields 处理
				}
				// 使用支持包目录的版本
				extractParamsFromFuncWithPackage(fn, action, file, fset, packageDir)
				extractReturnsFromFunc(fn, action, file, fset)
				found = true
				return false
			}
			return true
		})

		// 如果当前文件没找到，尝试在其他包中查找
		if !found {
			// 从 import 语句中找到函数所在的包
			packagePath := findPackageByFuncName(ident.Name, file)
			if packagePath != "" {
				handlerFile := findHandlerFile(packagePath, ident.Name)
				if handlerFile != "" {
					parseExternalHandler(handlerFile, ident.Name, action)
				}
			}
		}
	} else if fn, ok := handler.(*ast.FuncLit); ok {
		// 匿名函数
		// 获取当前文件所在目录，用于查找同一包内的类型
		packageDir := ""
		if routerFilePath != "" {
			packageDir = filepath.Dir(routerFilePath)
		}
		// 使用支持包目录的版本（如果包目录为空，函数内部会回退）
		extractParamsFromFuncBody(fn.Body, action, file, fset, packageDir)
		extractReturnsFromFuncBody(fn.Body, action, file, fset)
	}
	return nil
}

// findPackagePath 从 import 语句中找到包的完整路径
func findPackagePath(pkgName string, file *ast.File) string {
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)

		// 检查是否是项目内的包（通过路径前缀判断）
		// 支持任意路径，只要包含 internal/ 即可
		if strings.Contains(path, "/internal/") {
			// 提取包名（路径的最后一部分）
			parts := strings.Split(path, "/")
			if len(parts) > 0 && parts[len(parts)-1] == pkgName {
				return path
			}
		}
	}
	return ""
}

// findPackageByFuncName 根据函数名查找可能的包（通过搜索所有 import）
func findPackageByFuncName(funcName string, file *ast.File) string {
	// 遍历所有项目内的包，查找包含该函数的文件
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		// 支持任意包含 /internal/ 的包路径
		if strings.Contains(path, "/internal/") {
			handlerFile := findHandlerFile(path, funcName)
			if handlerFile != "" {
				return path
			}
		}
	}
	return ""
}

// findHandlerFile 查找 handler 函数所在的文件
func findHandlerFile(packagePath, funcName string) string {
	// 将包路径转换为文件系统路径
	// 支持任意格式的包路径，提取相对路径部分
	// 例如：carl-server/internal/app/login -> ./internal/app/login
	// 或者：github.com/user/project/internal/app/login -> 需要根据实际情况处理

	// 查找 "internal/" 在路径中的位置
	internalIdx := strings.Index(packagePath, "/internal/")
	if internalIdx == -1 {
		return ""
	}

	// 提取从 internal/ 开始的部分
	relativePath := packagePath[internalIdx+1:] // 去掉开头的 "/"
	dir := "./" + relativePath
	files, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".go") {
			continue
		}

		filePath := dir + "/" + file.Name()
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
		if err != nil {
			continue
		}

		// 检查是否包含该函数
		found := false
		ast.Inspect(node, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == funcName {
				found = true
				return false
			}
			return true
		})

		if found {
			return filePath
		}
	}

	return ""
}

// parseExternalHandler 解析外部包的 handler 函数
func parseExternalHandler(filePath, funcName string, action *ActionDoc) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return
	}

	// 获取包目录，用于查找同一包内的其他文件
	packageDir := filepath.Dir(filePath)

	// 查找函数定义
	ast.Inspect(node, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == funcName {
			// 提取参数和返回值，传入包目录以便查找同一包内的类型
			extractParamsFromFuncWithPackage(fn, action, node, fset, packageDir)
			extractReturnsFromFunc(fn, action, node, fset)

			// 提取函数注释作为描述
			if fn.Doc != nil {
				action.Description = extractComment(fn.Doc)
			}
			return false
		}
		return true
	})
}

// extractParamsFromFuncWithPackage 从函数中提取参数（支持查找同一包内的类型）
func extractParamsFromFuncWithPackage(fn *ast.FuncDecl, action *ActionDoc, file *ast.File, fset *token.FileSet, packageDir string) {
	if fn.Body == nil {
		return
	}
	extractParamsFromFuncBody(fn.Body, action, file, fset, packageDir)
}

// extractStructFieldsWithPackage 提取结构体字段（支持查找同一包内的类型）
func extractStructFieldsWithPackage(typeExpr ast.Expr, action *ActionDoc, file *ast.File, packageDir string) {
	// 处理指针类型：*model.QuestionsGroup
	if star, ok := typeExpr.(*ast.StarExpr); ok {
		// 递归处理指针指向的类型
		extractStructFieldsWithPackage(star.X, action, file, packageDir)
		return
	}

	// 处理复合字面量：struct{...}{}（匿名结构体）
	if compLit, ok := typeExpr.(*ast.CompositeLit); ok {
		if compLit.Type != nil {
			extractStructFieldsWithPackage(compLit.Type, action, file, packageDir)
		}
		return
	}

	// 处理类型标识符：在当前文件或同一包的其他文件中查找
	if ident, ok := typeExpr.(*ast.Ident); ok {
		// 先在当前文件中查找
		found := false
		ast.Inspect(file, func(n ast.Node) bool {
			if ts, ok := n.(*ast.TypeSpec); ok && ts.Name.Name == ident.Name {
				if st, ok := ts.Type.(*ast.StructType); ok {
					extractFieldsFromStruct(st, action)
					found = true
					return false
				}
			}
			return true
		})

		// 如果当前文件没找到，在同一包的其他文件中查找
		if !found && packageDir != "" {
			files, err := os.ReadDir(packageDir)
			if err == nil {
				for _, f := range files {
					if f.IsDir() || !strings.HasSuffix(f.Name(), ".go") {
						continue
					}
					otherFilePath := filepath.Join(packageDir, f.Name())
					fset := token.NewFileSet()
					otherNode, err := parser.ParseFile(fset, otherFilePath, nil, parser.ParseComments)
					if err != nil {
						continue
					}
					// 确保是同一个包
					if otherNode.Name != nil && file.Name != nil && otherNode.Name.Name == file.Name.Name {
						ast.Inspect(otherNode, func(n ast.Node) bool {
							if ts, ok := n.(*ast.TypeSpec); ok && ts.Name.Name == ident.Name {
								if st, ok := ts.Type.(*ast.StructType); ok {
									extractFieldsFromStruct(st, action)
									found = true
									return false
								}
							}
							return true
						})
						if found {
							break
						}
					}
				}
			}
		}
		return
	}

	// 处理 SelectorExpr：model.QuestionsGroup
	if sel, ok := typeExpr.(*ast.SelectorExpr); ok {
		// 获取包名和类型名
		var pkgName, typeName string
		if pkgIdent, ok := sel.X.(*ast.Ident); ok {
			pkgName = pkgIdent.Name
			typeName = sel.Sel.Name
		}

		if pkgName != "" && typeName != "" {
			// 从 import 语句中找到包的完整路径
			packagePath := findPackagePath(pkgName, file)
			if packagePath != "" {
				// 查找类型定义
				typeFile := findTypeFile(packagePath, typeName)
				if typeFile != "" {
					parseExternalType(typeFile, typeName, action)
				}
			}
		}
		return
	}

	// 直接的结构体类型（包括匿名结构体）
	if st, ok := typeExpr.(*ast.StructType); ok {
		extractFieldsFromStruct(st, action)
	}
}

// findTypeFile 查找类型定义所在的文件
func findTypeFile(packagePath, typeName string) string {
	// 将包路径转换为文件系统路径
	// 查找 "internal/" 在路径中的位置
	internalIdx := strings.Index(packagePath, "/internal/")
	if internalIdx == -1 {
		return ""
	}

	// 提取从 internal/ 开始的部分
	relativePath := packagePath[internalIdx+1:] // 去掉开头的 "/"
	dir := "./" + relativePath

	files, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".go") {
			continue
		}

		filePath := dir + "/" + file.Name()
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
		if err != nil {
			continue
		}

		// 检查是否包含该类型定义
		found := false
		ast.Inspect(node, func(n ast.Node) bool {
			if ts, ok := n.(*ast.TypeSpec); ok && ts.Name.Name == typeName {
				if _, ok := ts.Type.(*ast.StructType); ok {
					found = true
					return false
				}
			}
			return true
		})

		if found {
			return filePath
		}
	}

	return ""
}

// parseExternalType 解析外部包的类型定义
func parseExternalType(filePath, typeName string, action *ActionDoc) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return
	}

	// 查找类型定义
	ast.Inspect(node, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok && ts.Name.Name == typeName {
			if st, ok := ts.Type.(*ast.StructType); ok {
				extractFieldsFromStruct(st, action)
				return false
			}
		}
		return true
	})
}

// parseExternalTypeWithPrefix 解析外部包的类型定义，并在字段名前添加前缀
func parseExternalTypeWithPrefix(filePath, typeName string, action *ActionDoc, prefix string) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return
	}

	// 查找类型定义
	ast.Inspect(node, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok && ts.Name.Name == typeName {
			if st, ok := ts.Type.(*ast.StructType); ok {
				extractFieldsFromStructWithPrefix(st, action, prefix)
				return false
			}
		}
		return true
	})
}

// extractFieldsFromStructWithPrefix 从结构体中提取字段，并在字段名前添加前缀
func extractFieldsFromStructWithPrefix(st *ast.StructType, action *ActionDoc, prefix string) {
	if st.Fields == nil {
		return
	}

	for _, field := range st.Fields.List {
		if len(field.Names) == 0 {
			continue
		}

		fieldName := field.Names[0].Name
		fieldType := getTypeString(field.Type)
		jsonTag := extractJSONTag(field.Tag)

		// 跳过 json:"-" 的字段（这些字段不会出现在 JSON 中）
		if jsonTag == "-" {
			continue
		}

		// 检查 gorm 标签，跳过自增主键和带 CURRENT_TIMESTAMP 默认值的字段
		if shouldSkipGormField(field.Tag) {
			continue
		}

		required := !isOptional(field.Type, field.Tag)

		// 使用 JSON 标签作为参数名，如果没有则使用字段名
		paramName := jsonTag
		if paramName == "" {
			paramName = fieldName
		}

		// 添加前缀（如 "page."）
		paramName = prefix + "." + paramName

		// 提取注释：优先使用行内注释（field.Comment），如果没有则使用上方注释（field.Doc）
		description := extractComment(field.Comment)
		if description == "" {
			description = extractComment(field.Doc)
		}

		// 检查是否已存在（避免重复）
		exists := false
		for _, p := range action.Params {
			if p.Name == paramName {
				exists = true
				break
			}
		}
		if !exists {
			action.Params = append(action.Params, ParamField{
				Name:        paramName,
				Type:        fieldType,
				Required:    required,
				Description: description,
			})
		}
	}
}

// extractParamsFromFuncBody 从函数体中提取参数
// packageDir 是可选的包目录，用于查找同一包内的类型定义
func extractParamsFromFuncBody(body *ast.BlockStmt, action *ActionDoc, file *ast.File, fset *token.FileSet, packageDir string) {
	// 查找 BindingJson、BindingValidateJson 或 GetJson 调用
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		// 查找 a.BindingJson(&req)、a.BindingValidateJson(&req) 或 a.GetJson(&req) 调用
		if (sel.Sel.Name == "BindingJson" || sel.Sel.Name == "BindingValidateJson" || sel.Sel.Name == "GetJson") && len(call.Args) > 0 {
			if unary, ok := call.Args[0].(*ast.UnaryExpr); ok && unary.Op == token.AND {
				if ident, ok := unary.X.(*ast.Ident); ok {
					// 查找变量声明（支持 var 和 := 两种方式）
					var varType ast.Expr
					ast.Inspect(body, func(n ast.Node) bool {
						// 处理 var 声明：var req Type 或 var req = Type{}
						if decl, ok := n.(*ast.GenDecl); ok && decl.Tok == token.VAR {
							for _, spec := range decl.Specs {
								if vs, ok := spec.(*ast.ValueSpec); ok {
									for _, name := range vs.Names {
										if name.Name == ident.Name {
											if vs.Type != nil {
												varType = vs.Type
											} else if len(vs.Values) > 0 {
												// 推断类型：var req = Type{}
												// 对于匿名结构体，vs.Values[0] 是 *ast.CompositeLit
												if compLit, ok := vs.Values[0].(*ast.CompositeLit); ok {
													if compLit.Type != nil {
														varType = compLit.Type
													}
												} else {
													varType = vs.Values[0]
												}
											}
											return false
										}
									}
								}
							}
						}
						// 处理短变量声明：req := Type{} 或 req := struct{...}{}
						if assign, ok := n.(*ast.AssignStmt); ok && assign.Tok == token.DEFINE {
							for i, lhs := range assign.Lhs {
								if lhsIdent, ok := lhs.(*ast.Ident); ok && lhsIdent.Name == ident.Name {
									if i < len(assign.Rhs) {
										// 处理匿名结构体：req := struct{...}{}
										if compLit, ok := assign.Rhs[i].(*ast.CompositeLit); ok {
											if compLit.Type != nil {
												varType = compLit.Type
											}
										} else {
											// 处理普通类型：req := Type{}
											varType = assign.Rhs[i]
										}
									}
									return false
								}
							}
						}
						return true
					})

					if varType != nil {
						// 传入包目录（如果提供）
						if packageDir != "" {
							extractStructFields(varType, action, file, packageDir)
						} else {
							extractStructFields(varType, action, file)
						}
					}
				}
			}
		}

		// 查找 a.BindingJsonPath(&codes, "codes") 调用
		if sel.Sel.Name == "BindingJsonPath" && len(call.Args) >= 2 {
			if unary, ok := call.Args[0].(*ast.UnaryExpr); ok && unary.Op == token.AND {
				if ident, ok := unary.X.(*ast.Ident); ok {
					// 提取第二个参数作为参数名（JSON 路径）
					pathName := extractStringLiteral(call.Args[1])
					if pathName == "" {
						pathName = "path"
					}

					// 查找变量声明以获取类型
					var varType ast.Expr
					ast.Inspect(body, func(n ast.Node) bool {
						// 处理 var 声明：var codes []string
						if decl, ok := n.(*ast.GenDecl); ok && decl.Tok == token.VAR {
							for _, spec := range decl.Specs {
								if vs, ok := spec.(*ast.ValueSpec); ok {
									for _, name := range vs.Names {
										if name.Name == ident.Name {
											if vs.Type != nil {
												varType = vs.Type
											}
											return false
										}
									}
								}
							}
						}
						// 处理短变量声明：codes := []string{}
						if assign, ok := n.(*ast.AssignStmt); ok && assign.Tok == token.DEFINE {
							for i, lhs := range assign.Lhs {
								if lhsIdent, ok := lhs.(*ast.Ident); ok && lhsIdent.Name == ident.Name {
									if i < len(assign.Rhs) {
										if compLit, ok := assign.Rhs[i].(*ast.CompositeLit); ok {
											if compLit.Type != nil {
												varType = compLit.Type
											}
										} else {
											varType = assign.Rhs[i]
										}
									}
									return false
								}
							}
						}
						return true
					})

					// 如果找到了类型，提取字段（使用 pathName 作为前缀）
					if varType != nil {
						// 检查是否是结构体类型，如果是则提取字段
						if st, ok := varType.(*ast.StructType); ok {
							// 直接提取结构体字段，使用 pathName 作为前缀
							extractFieldsFromStructWithPrefix(st, action, pathName)
						} else if sel, ok := varType.(*ast.SelectorExpr); ok {
							// 处理外部包的类型（如 model.XXX）
							var pkgName, typeName string
							if pkgIdent, ok := sel.X.(*ast.Ident); ok {
								pkgName = pkgIdent.Name
								typeName = sel.Sel.Name
							}
							if pkgName != "" && typeName != "" {
								// 从 import 语句中找到包的完整路径
								packagePath := findPackagePath(pkgName, file)
								if packagePath != "" {
									// 查找类型定义
									typeFile := findTypeFile(packagePath, typeName)
									if typeFile != "" {
										parseExternalTypeWithPrefix(typeFile, typeName, action, pathName)
									}
								}
							}
						} else if ident, ok := varType.(*ast.Ident); ok {
							// 处理当前包的类型，需要查找类型定义
							// 如果提供了包目录，使用支持包目录的版本
							if packageDir != "" {
								// 在同一包的其他文件中查找类型定义
								files, err := os.ReadDir(packageDir)
								if err == nil {
									for _, f := range files {
										if f.IsDir() || !strings.HasSuffix(f.Name(), ".go") {
											continue
										}
										otherFilePath := filepath.Join(packageDir, f.Name())
										fset := token.NewFileSet()
										otherNode, err := parser.ParseFile(fset, otherFilePath, nil, parser.ParseComments)
										if err != nil {
											continue
										}
										// 确保是同一个包
										if otherNode.Name != nil && file.Name != nil && otherNode.Name.Name == file.Name.Name {
											ast.Inspect(otherNode, func(n ast.Node) bool {
												if ts, ok := n.(*ast.TypeSpec); ok && ts.Name.Name == ident.Name {
													if st, ok := ts.Type.(*ast.StructType); ok {
														extractFieldsFromStructWithPrefix(st, action, pathName)
														return false
													}
												}
												return true
											})
										}
									}
								}
							} else {
								// 在当前文件中查找类型定义
								ast.Inspect(file, func(n ast.Node) bool {
									if ts, ok := n.(*ast.TypeSpec); ok && ts.Name.Name == ident.Name {
										if st, ok := ts.Type.(*ast.StructType); ok {
											extractFieldsFromStructWithPrefix(st, action, pathName)
											return false
										}
									}
									return true
								})
							}
						} else {
							// 其他类型（如数组、指针等），添加一个通用参数
							typeStr := getTypeString(varType)
							exists := false
							for _, p := range action.Params {
								if p.Name == pathName {
									exists = true
									break
								}
							}
							if !exists {
								action.Params = append(action.Params, ParamField{
									Name:     pathName,
									Type:     typeStr,
									Required: true,
								})
							}
						}
					} else {
						// 如果找不到类型，至少添加参数名
						exists := false
						for _, p := range action.Params {
							if p.Name == pathName {
								exists = true
								break
							}
						}
						if !exists {
							action.Params = append(action.Params, ParamField{
								Name:     pathName,
								Type:     "any",
								Required: true,
							})
						}
					}
				}
			}
		}

		// 查找 a.GetPagination() 或 a.GetMaxPagination() 调用
		// GetPagination/GetMaxPagination 从请求参数中的 "page" 路径获取分页对象，类似 BindingJsonPath
		if sel.Sel.Name == "GetPagination" || sel.Sel.Name == "GetMaxPagination" {
			// 检查是否是 a.GetPagination() 调用（sel.X 应该是函数参数 a）
			if sel.X == nil {
				return true
			}
			// 只处理 sel.X 是标识符的情况（通常是函数参数名，如 "a"）
			_, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}

			// 查找 ws.Page 结构体定义并提取字段
			// 从 import 语句中找到 ws 包的完整路径
			wsPackagePath := findPackagePath("ws", file)
			if wsPackagePath != "" {
				// 查找 Page 类型定义
				typeFile := findTypeFile(wsPackagePath, "Page")
				if typeFile != "" {
					// 解析 Page 结构体并提取字段，使用 "page." 作为前缀
					parseExternalTypeWithPrefix(typeFile, "Page", action, "page")
				}
			} else {
				// 如果找不到 ws 包，使用默认的分页参数
				paginationParams := []struct {
					name string
					typ  string
				}{
					{"page.current", "int"},
					{"page.pageSize", "int"},
				}

				for _, param := range paginationParams {
					// 检查是否已存在（避免重复）
					exists := false
					for _, p := range action.Params {
						if p.Name == param.name {
							exists = true
							break
						}
					}
					if !exists {
						action.Params = append(action.Params, ParamField{
							Name:     param.name,
							Type:     param.typ,
							Required: false, // 分页参数通常有默认值，所以是可选的
						})
					}
				}
			}
		}

		// 查找 a.Get(), a.GetInt() 等调用
		if sel.Sel.Name == "Get" || sel.Sel.Name == "GetInt" || sel.Sel.Name == "GetBool" || sel.Sel.Name == "GetId" {
			// 检查是否是 a.Get() 调用（sel.X 应该是函数参数 a）
			if sel.X == nil {
				return true
			}
			// 只处理 sel.X 是标识符的情况（通常是函数参数名，如 "a"）
			_, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}

			if len(call.Args) > 0 {
				key := extractStringLiteral(call.Args[0])
				if key != "" {
					paramType := "string"
					switch sel.Sel.Name {
					case "GetInt":
						paramType = "int"
					case "GetBool":
						paramType = "bool"
					case "GetId":
						paramType = "uint"
					}

					// 检查是否已存在（避免重复）
					exists := false
					for _, p := range action.Params {
						if p.Name == key {
							exists = true
							break
						}
					}
					if !exists {
						action.Params = append(action.Params, ParamField{
							Name:     key,
							Type:     paramType,
							Required: true,
						})
					}
				}
			}
		}

		return true
	})
}

// extractStructFields 提取结构体字段（在当前文件或跨包中查找）
// packageDir 是可选的包目录，如果提供则会在同一包的其他文件中查找类型
func extractStructFields(typeExpr ast.Expr, action *ActionDoc, file *ast.File, packageDir ...string) {
	var dir string
	if len(packageDir) > 0 {
		dir = packageDir[0]
	}

	// 如果提供了包目录，使用支持包目录的版本
	if dir != "" {
		extractStructFieldsWithPackage(typeExpr, action, file, dir)
		return
	}

	// 处理指针类型：*model.QuestionsGroup
	if star, ok := typeExpr.(*ast.StarExpr); ok {
		// 递归处理指针指向的类型
		extractStructFields(star.X, action, file)
		return
	}

	// 处理复合字面量：struct{...}{}（匿名结构体）
	if compLit, ok := typeExpr.(*ast.CompositeLit); ok {
		if compLit.Type != nil {
			extractStructFields(compLit.Type, action, file)
		}
		return
	}

	// 处理类型标识符：在当前文件中查找
	// 注意：如果类型在同一包的其他文件中，这里可能找不到
	// 但跨包的类型可以通过 SelectorExpr 路径找到
	if ident, ok := typeExpr.(*ast.Ident); ok {
		ast.Inspect(file, func(n ast.Node) bool {
			if ts, ok := n.(*ast.TypeSpec); ok && ts.Name.Name == ident.Name {
				if st, ok := ts.Type.(*ast.StructType); ok {
					extractFieldsFromStruct(st, action)
					return false
				}
			}
			return true
		})
		// 如果当前文件没找到，且提供了包目录，会在 extractStructFieldsWithPackage 中查找同一包的其他文件
		return
	}

	// 处理 SelectorExpr：model.QuestionsGroup
	if sel, ok := typeExpr.(*ast.SelectorExpr); ok {
		// 获取包名和类型名
		var pkgName, typeName string
		if pkgIdent, ok := sel.X.(*ast.Ident); ok {
			pkgName = pkgIdent.Name
			typeName = sel.Sel.Name
		}

		if pkgName != "" && typeName != "" {
			// 从 import 语句中找到包的完整路径
			packagePath := findPackagePath(pkgName, file)
			if packagePath != "" {
				// 查找类型定义
				typeFile := findTypeFile(packagePath, typeName)
				if typeFile != "" {
					parseExternalType(typeFile, typeName, action)
				}
			}
		}
		return
	}

	// 直接的结构体类型（包括匿名结构体）
	if st, ok := typeExpr.(*ast.StructType); ok {
		extractFieldsFromStruct(st, action)
	}
}

// extractFieldsFromStruct 从结构体中提取字段
func extractFieldsFromStruct(st *ast.StructType, action *ActionDoc) {
	if st.Fields == nil {
		return
	}

	for _, field := range st.Fields.List {
		if len(field.Names) == 0 {
			continue
		}

		fieldName := field.Names[0].Name
		fieldType := getTypeString(field.Type)
		jsonTag := extractJSONTag(field.Tag)

		// 跳过 json:"-" 的字段（这些字段不会出现在 JSON 中）
		if jsonTag == "-" {
			continue
		}

		// 检查 gorm 标签，跳过自增主键和带 CURRENT_TIMESTAMP 默认值的字段
		if shouldSkipGormField(field.Tag) {
			continue
		}

		required := !isOptional(field.Type, field.Tag)

		// 使用 JSON 标签作为参数名
		paramName := jsonTag
		if paramName == "" {
			paramName = fieldName
		}

		// 提取注释：优先使用行内注释（field.Comment），如果没有则使用上方注释（field.Doc）
		description := extractComment(field.Comment)
		if description == "" {
			description = extractComment(field.Doc)
		}

		action.Params = append(action.Params, ParamField{
			Name:        paramName,
			Type:        fieldType,
			Required:    required,
			Description: description,
		})
	}
}

// getTypeString 获取类型字符串
func getTypeString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return fmt.Sprintf("%s.%s", t.X, t.Sel.Name)
	case *ast.ArrayType:
		return "[]" + getTypeString(t.Elt)
	case *ast.MapType:
		return fmt.Sprintf("map[%s]%s", getTypeString(t.Key), getTypeString(t.Value))
	case *ast.StarExpr:
		return "*" + getTypeString(t.X)
	default:
		return "any"
	}
}

// extractJSONTag 提取 JSON 标签
func extractJSONTag(tag *ast.BasicLit) string {
	if tag == nil {
		return ""
	}

	tagValue := strings.Trim(tag.Value, "`")
	parts := strings.Fields(tagValue)
	for _, part := range parts {
		if strings.HasPrefix(part, "json:") {
			jsonValue := strings.TrimPrefix(part, "json:")
			jsonValue = strings.Trim(jsonValue, `"`)
			// 处理 omitempty
			if idx := strings.Index(jsonValue, ","); idx > 0 {
				jsonValue = jsonValue[:idx]
			}
			return jsonValue
		}
	}
	return ""
}

// extractGormTag 提取 gorm 标签值
func extractGormTag(tag *ast.BasicLit) string {
	if tag == nil {
		return ""
	}

	tagValue := strings.Trim(tag.Value, "`")

	// 查找 gorm: 前缀
	gormPrefix := "gorm:"
	idx := strings.Index(tagValue, gormPrefix)
	if idx == -1 {
		return ""
	}

	// 跳过 "gorm:" 前缀
	start := idx + len(gormPrefix)
	if start >= len(tagValue) {
		return ""
	}

	// 查找引号开始位置
	if tagValue[start] != '"' {
		return ""
	}
	start++ // 跳过开始的引号

	// 查找引号结束位置（需要处理转义的引号）
	end := start
	inEscape := false
	for end < len(tagValue) {
		if inEscape {
			inEscape = false
			end++
			continue
		}
		if tagValue[end] == '\\' {
			inEscape = true
			end++
			continue
		}
		if tagValue[end] == '"' {
			break
		}
		end++
	}

	if end >= len(tagValue) {
		return ""
	}

	// 提取引号内的内容
	return tagValue[start:end]
}

// shouldSkipGormField 检查是否应该跳过该字段（自增主键或带 CURRENT_TIMESTAMP 默认值）
func shouldSkipGormField(tag *ast.BasicLit) bool {
	gormTag := extractGormTag(tag)
	if gormTag == "" {
		return false
	}

	// 检查是否是自增主键：包含 primaryKey 和 autoIncrement:true
	hasPrimaryKey := strings.Contains(gormTag, "primaryKey")
	hasAutoIncrement := strings.Contains(gormTag, "autoIncrement:true") || strings.Contains(gormTag, "autoIncrement")
	if hasPrimaryKey && hasAutoIncrement {
		return true
	}

	// 检查是否有 CURRENT_TIMESTAMP 默认值
	// 支持多种格式：default:CURRENT_TIMESTAMP, default:now(), default:current_timestamp 等
	if strings.Contains(gormTag, "default:CURRENT_TIMESTAMP") ||
		strings.Contains(gormTag, "default:current_timestamp") ||
		strings.Contains(gormTag, "default:now()") ||
		strings.Contains(gormTag, "default:CURRENT_TIMESTAMP()") {
		return true
	}

	return false
}

// isOptional 判断字段是否可选
func isOptional(expr ast.Expr, tag *ast.BasicLit) bool {
	// 指针类型是可选的
	if _, ok := expr.(*ast.StarExpr); ok {
		return true
	}

	// 标签中有 omitempty
	if tag != nil {
		tagValue := strings.Trim(tag.Value, "`")
		return strings.Contains(tagValue, "omitempty")
	}

	return false
}

// extractComment 提取注释
func extractComment(commentGroup *ast.CommentGroup) string {
	if commentGroup == nil {
		return ""
	}

	var comments []string
	for _, comment := range commentGroup.List {
		text := strings.TrimSpace(comment.Text)
		text = strings.TrimPrefix(text, "//")
		text = strings.TrimSpace(text)
		if text != "" {
			comments = append(comments, text)
		}
	}
	return strings.Join(comments, " ")
}

// extractReturnsFromFunc 从函数中提取返回值
func extractReturnsFromFunc(fn *ast.FuncDecl, action *ActionDoc, file *ast.File, fset *token.FileSet) {
	if fn.Body == nil {
		return
	}
	extractReturnsFromFuncBody(fn.Body, action, file, fset)
}

// extractReturnsFromFuncBody 从函数体中提取返回值
func extractReturnsFromFuncBody(body *ast.BlockStmt, action *ActionDoc, file *ast.File, fset *token.FileSet) {
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		// 检查是否是 a.Send(), a.SendOk(), a.SendCode() 等调用
		// sel.X 应该是函数参数 a (*ws.Context)，通常是 *ast.Ident 类型
		if sel.X == nil {
			return true
		}

		// 只处理 sel.X 是标识符的情况（通常是函数参数名，如 "a"）
		// 这样可以避免处理其他类型的调用（如 obj.method()）
		_, ok = sel.X.(*ast.Ident)
		if !ok {
			return true
		}

		switch sel.Sel.Name {
		case "Send":
			action.Returns.HasData = true
			if len(call.Args) > 0 {
				action.Returns.SuccessType = getTypeString(call.Args[0])
			}
		case "SendOk":
			action.Returns.HasData = false
		case "SendCode":
			action.Returns.HasData = false
			if len(call.Args) >= 2 {
				// 提取错误码（支持字面量和变量）
				code := extractIntValue(call.Args[0])
				if code > 0 {
					// 提取错误消息（支持字面量和变量）
					msg := extractStringValue(call.Args[1])
					// 如果消息为空，使用默认描述
					if msg == "" {
						msg = "错误"
					}
					// 检查是否已存在相同的错误码
					exists := false
					for i, errCode := range action.Returns.ErrorCodes {
						if errCode.Code == code {
							// 如果已存在，保留更详细的描述（较长的消息）
							if len(msg) > len(errCode.Message) {
								action.Returns.ErrorCodes[i].Message = msg
							}
							exists = true
							break
						}
					}
					// 如果不存在，添加新的错误码
					if !exists {
						action.Returns.ErrorCodes = append(action.Returns.ErrorCodes, ErrorCode{
							Code:    code,
							Message: msg,
						})
					}
				}
			}
		default:
			// 不是我们要处理的方法，继续遍历
			return true
		}

		return true
	})
}

// extractIntLiteral 提取整数字面量
func extractIntLiteral(expr ast.Expr) (int, bool) {
	if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.INT {
		var val int
		_, err := fmt.Sscanf(lit.Value, "%d", &val)
		return val, err == nil
	}
	return 0, false
}

// extractIntValue 提取整数值（支持字面量和标识符）
func extractIntValue(expr ast.Expr) int {
	// 首先尝试提取字面量
	if val, ok := extractIntLiteral(expr); ok {
		return val
	}
	// 如果是标识符（变量），无法确定值，返回 0 表示未知
	if _, ok := expr.(*ast.Ident); ok {
		return 0
	}
	return 0
}

// extractStringValue 提取字符串值（支持字面量和标识符）
func extractStringValue(expr ast.Expr) string {
	// 首先尝试提取字面量
	if msg := extractStringLiteral(expr); msg != "" {
		return msg
	}
	// 如果是方法调用（如 err.Error()），尝试识别
	if call, ok := expr.(*ast.CallExpr); ok {
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if sel.Sel.Name == "Error" {
				return "错误信息"
			}
		}
	}
	// 如果是标识符（变量），返回变量名作为提示
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}
