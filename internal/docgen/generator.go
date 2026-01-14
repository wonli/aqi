package docgen

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// GenerateMarkdown 生成 Markdown 文档
func GenerateMarkdown(routerFiles []RouterFile, outputPath string) error {
	var buf strings.Builder

	// 获取元数据
	metadata := getMetadata()

	// 写入文档头部
	buf.WriteString(fmt.Sprintf("# %s API 命令文档", metadata.ProjectName))
	if metadata.Version != "" {
		buf.WriteString(fmt.Sprintf(" (v%s)", metadata.Version))
	}
	buf.WriteString("\n\n")

	// 项目描述
	if metadata.Description != "" {
		buf.WriteString(fmt.Sprintf("%s\n\n", metadata.Description))
	}

	// 元数据信息
	if metadata.Author != "" || metadata.Contact != "" {
		buf.WriteString("**项目信息：**\n\n")
		if metadata.Author != "" {
			buf.WriteString(fmt.Sprintf("- 作者：%s\n", metadata.Author))
		}
		if metadata.Contact != "" {
			buf.WriteString(fmt.Sprintf("- 联系方式：%s\n", metadata.Contact))
		}
		buf.WriteString("\n")
	}

	// 使用说明
	buf.WriteString("**WebSocket 客户端统一请求格式：**\n\n")
	buf.WriteString("- 连接参数：`token`（登录后动作必需）、`clientId`（app 客户端识别）、`appId`、`platform`\n")
	buf.WriteString("- 消息体：`{\"action\":\"<动作名称>\", \"params\": { ... }}`\n")
	buf.WriteString("- 返回值：统一为 JSON（包含 `code`/`msg`/`data` 等），具体字段以对应模块实现为准。\n\n")

	fmt.Printf("开始生成文档，共 %d 个路由文件\n", len(routerFiles))

	// 按路由文件分组（现在每个文件生成独立文档，所以这里应该只有一个文件）
	for _, rf := range routerFiles {
		if len(rf.Actions) == 0 {
			fmt.Printf("跳过空的路由文件: %s\n", rf.FileName)
			continue
		}

		fmt.Printf("生成文档: %s (%d 个 action)\n", rf.FileName, len(rf.Actions))
		// 路由文件标题
		fileTitle := getFileTitle(rf.FileName)
		buf.WriteString(fmt.Sprintf("## %s\n\n", fileTitle))

		// 按中间件组合分组
		middlewareGroups := groupByMiddleware(rf.Actions)

		// 按中间件组排序（先无需登录，后需要登录）
		var groupKeys []string
		for key := range middlewareGroups {
			groupKeys = append(groupKeys, key)
		}
		sort.Slice(groupKeys, func(i, j int) bool {
			// 需要登录的排在后面
			hasAuthI := strings.Contains(groupKeys[i], "登录")
			hasAuthJ := strings.Contains(groupKeys[j], "登录")
			if hasAuthI != hasAuthJ {
				return !hasAuthI
			}
			return groupKeys[i] < groupKeys[j]
		})

		for _, groupKey := range groupKeys {
			actions := middlewareGroups[groupKey]
			if len(actions) == 0 {
				continue
			}

			// 中间件组标题
			buf.WriteString(fmt.Sprintf("### %s\n\n", groupKey))

			// 按功能分类分组
			categories := groupByCategory(actions)

			// 按分类排序
			var categoryKeys []string
			for key := range categories {
				categoryKeys = append(categoryKeys, key)
			}
			sort.Strings(categoryKeys)

			for _, categoryKey := range categoryKeys {
				categoryActions := categories[categoryKey]
				if len(categoryActions) == 0 {
					continue
				}

				// 如果某分类仅包含一个 action，则不再额外分组，直接输出
				if len(categoryActions) == 1 {
					action := categoryActions[0]
					actionLevel := 4 // 精简层级
					buf.WriteString(formatAction(action, actionLevel))
					buf.WriteString("\n\n")
					continue
				}

				// 分类标题（多个 action 时才显示分类标题）
				if len(categories) > 1 {
					buf.WriteString(fmt.Sprintf("#### %s\n\n", categoryKey))
				}

				// 按 action 名称排序
				sort.Slice(categoryActions, func(i, j int) bool {
					return categoryActions[i].Name < categoryActions[j].Name
				})

				for _, action := range categoryActions {
					// 根据层级深度决定 Action 标题级别
					// 如果有分类，使用 #####，否则使用 ####
					actionLevel := 5 // #####
					if len(categories) <= 1 {
						actionLevel = 4 // ####
					}
					buf.WriteString(formatAction(action, actionLevel))
					buf.WriteString("\n")
				}

				buf.WriteString("\n")
			}
		}
	}

	// 写入文件
	return os.WriteFile(outputPath, []byte(buf.String()), 0644)
}

// getFileTitle 获取文件标题
func getFileTitle(fileName string) string {
	// 去掉 .go 后缀，作为标题
	baseName := strings.TrimSuffix(fileName, ".go")
	// 将下划线或连字符转换为空格，并首字母大写
	title := strings.ReplaceAll(baseName, "_", " ")
	title = strings.ReplaceAll(title, "-", " ")
	// 简单的首字母大写处理
	if len(title) > 0 {
		title = strings.ToUpper(title[:1]) + title[1:]
	}
	return title + " API"
}

// groupByMiddleware 按中间件组合分组
func groupByMiddleware(actions []ActionDoc) map[string][]ActionDoc {
	groups := make(map[string][]ActionDoc)

	for _, action := range actions {
		groupKey := getMiddlewareGroupName(action)
		groups[groupKey] = append(groups[groupKey], action)
	}

	return groups
}

// getMiddlewareGroupName 获取中间件组名称（从配置中读取）
func getMiddlewareGroupName(action ActionDoc) string {
	if globalCategoryConfig == nil {
		// 如果配置未加载，使用默认值
		if len(action.MiddlewareChain) > 0 {
			mwNames := strings.Join(action.MiddlewareChain, ", ")
			return mwNames
		}
		return "默认"
	}

	mg := globalCategoryConfig.MiddlewareGroup

	// 使用统一的 MiddlewareMap
	if len(mg.MiddlewareMap) > 0 {
		// 1. 优先匹配中间件链（更具体，优先级更高）
		if len(action.MiddlewareChain) > 0 {
			// 使用下划线分隔的小写格式（如 recovery_app_auth）
			chainKey := strings.ToLower(strings.Join(action.MiddlewareChain, "_"))
			if chainName, ok := mg.MiddlewareMap[chainKey]; ok {
				return chainName
			}
		}

		// 2. 匹配中间件组变量名
		if action.MiddlewareGroup != "" {
			if groupName, ok := mg.MiddlewareMap[action.MiddlewareGroup]; ok {
				return groupName
			}
		}
	}

	// 5. 使用默认模板（基于中间件链）
	if len(action.MiddlewareChain) > 0 {
		mwNames := strings.Join(action.MiddlewareChain, ", ")
		return fmt.Sprintf(mg.DefaultTemplate, mwNames)
	}

	// 6. 无中间件
	return mg.NoMiddleware
}

// getAuthRequirement 获取权限要求描述（根据中间件组名称）
func getAuthRequirement(action ActionDoc) string {
	// 直接使用中间件组名称作为权限要求
	groupName := getMiddlewareGroupName(action)
	return groupName
}

// groupByCategory 按功能分类分组
func groupByCategory(actions []ActionDoc) map[string][]ActionDoc {
	groups := make(map[string][]ActionDoc)

	for _, action := range actions {
		category := action.Category
		if category == "" {
			category = "其他"
		}
		groups[category] = append(groups[category], action)
	}

	return groups
}

// formatAction 格式化 action（非表格格式）
// level 表示标题层级（3=###, 4=####, 5=#####）
func formatAction(action ActionDoc, level int) string {
	var buf strings.Builder

	// 根据层级生成标题
	heading := strings.Repeat("#", level)
	buf.WriteString(fmt.Sprintf("%s `%s`\n\n", heading, action.Name))

	// 功能描述
	if action.Description != "" {
		buf.WriteString(fmt.Sprintf("**功能描述：** %s\n\n", action.Description))
	}

	// 参数说明
	params := formatParamsList(action.Params)
	if params != "" {
		buf.WriteString(fmt.Sprintf("**参数说明：**\n\n%s\n\n", params))
	} else {
		buf.WriteString("**参数说明：** 无\n\n")
	}

	// 返回值
	returns := formatReturns(action.Returns)
	buf.WriteString(fmt.Sprintf("**返回值：** %s\n\n", returns))

	// 使用示例
	example := formatExample(action)
	// 去掉反引号（如果有），因为我们要放在代码块中
	example = strings.Trim(example, "`")
	buf.WriteString(fmt.Sprintf("**使用示例：**\n\n```json\n%s\n```\n\n", example))

	// 权限要求（从配置中读取）
	authReq := getAuthRequirement(action)
	buf.WriteString(fmt.Sprintf("**权限要求：** %s\n\n", authReq))

	// 错误码
	if len(action.Returns.ErrorCodes) > 0 {
		buf.WriteString(formatErrorCodes(action.Returns.ErrorCodes))
		buf.WriteString("\n")
	}

	return buf.String()
}

// formatParamsList 格式化参数列表（用于非表格格式）
func formatParamsList(params []ParamField) string {
	if len(params) == 0 {
		return ""
	}

	var buf strings.Builder
	for _, p := range params {
		required := "必需"
		if !p.Required {
			required = "可选"
		}
		typeInfo := p.Type
		if typeInfo == "" {
			typeInfo = "any"
		}
		desc := p.Description
		if desc == "" {
			desc = "-"
		}
		buf.WriteString(fmt.Sprintf("- `%s` (%s, %s): %s\n", p.Name, typeInfo, required, desc))
	}

	return buf.String()
}

// formatParams 格式化参数说明
func formatParams(params []ParamField) string {
	if len(params) == 0 {
		return ""
	}

	var parts []string
	for _, p := range params {
		part := fmt.Sprintf("`%s`", p.Name)
		if p.Type != "" {
			part += fmt.Sprintf(" (%s)", p.Type)
		}
		if !p.Required {
			part += " 可选"
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, ", ")
}

// formatReturns 格式化返回值
func formatReturns(returns ReturnType) string {
	if returns.HasData {
		if returns.SuccessType != "" && returns.SuccessType != "any" {
			return fmt.Sprintf("数据对象 (%s)", returns.SuccessType)
		}
		return "数据对象"
	}
	return "状态码"
}

// formatExample 格式化使用示例
func formatExample(action ActionDoc) string {
	var params []string
	for _, p := range action.Params {
		if p.Required {
			exampleValue := getExampleValue(p.Type)
			params = append(params, fmt.Sprintf("\"%s\": %s", p.Name, exampleValue))
		}
	}

	paramsStr := "{}"
	if len(params) > 0 {
		paramsStr = "{" + strings.Join(params, ", ") + "}"
	}

	return fmt.Sprintf("`{\"action\":\"%s\",\"params\":%s}`", action.Name, paramsStr)
}

// formatErrorCodes 格式化错误码列表
func formatErrorCodes(errorCodes []ErrorCode) string {
	if len(errorCodes) == 0 {
		return ""
	}

	var buf strings.Builder
	buf.WriteString("\n**错误码：**\n\n")

	// 按错误码排序
	sortedCodes := make([]ErrorCode, len(errorCodes))
	copy(sortedCodes, errorCodes)
	sort.Slice(sortedCodes, func(i, j int) bool {
		return sortedCodes[i].Code < sortedCodes[j].Code
	})

	for _, errCode := range sortedCodes {
		buf.WriteString(fmt.Sprintf("- `%d`: %s\n", errCode.Code, errCode.Message))
	}

	return buf.String()
}

// getExampleValue 获取示例值
func getExampleValue(typ string) string {
	switch typ {
	case "int", "uint", "int32", "uint32", "int64", "uint64":
		return "0"
	case "bool":
		return "true"
	case "[]string":
		return "[]"
	default:
		return "\"\""
	}
}

// JSONDocument JSON 文档结构
type JSONDocument struct {
	Metadata    metadataConfig   `json:"metadata"`
	GeneratedAt string           `json:"generatedAt"` // 文档生成时间（格式：2006-01-02 15:04:05）
	Info        JSONDocumentInfo `json:"info"`
	Files       []JSONRouterFile `json:"files"`
	Changelog   *ChangelogEntry  `json:"changelog,omitempty"` // 接口更新日志
}

// ChangelogEntry 更新日志条目
type ChangelogEntry struct {
	Version   string   `json:"version"`   // 版本号（commit 计数）
	Timestamp string   `json:"timestamp"` // 生成时间
	Added     []string `json:"added"`     // 新增的接口
	Removed   []string `json:"removed"`   // 删除的接口
}

// ApiSnapshot 接口快照（用于版本对比）
type ApiSnapshot struct {
	Version   string   `json:"version"`   // commit 计数
	Timestamp string   `json:"timestamp"` // 快照时间
	Actions   []string `json:"actions"`   // 接口名称列表
}

// JSONDocumentInfo 文档信息
type JSONDocumentInfo struct {
	RequestFormat  string `json:"requestFormat"`
	ResponseFormat string `json:"responseFormat"`
}

// JSONRouterFile 路由文件 JSON 结构
type JSONRouterFile struct {
	FileName         string                `json:"fileName"`
	Title            string                `json:"title"`
	MiddlewareGroups []JSONMiddlewareGroup `json:"middlewareGroups"`
}

// JSONMiddlewareGroup 中间件组 JSON 结构
type JSONMiddlewareGroup struct {
	Name       string         `json:"name"`
	Categories []JSONCategory `json:"categories"`
}

// JSONCategory 分类 JSON 结构
type JSONCategory struct {
	Name    string       `json:"name"`
	Actions []JSONAction `json:"actions"`
}

// JSONAction Action JSON 结构
type JSONAction struct {
	Name            string         `json:"name"`
	Description     string         `json:"description"`
	Params          []ParamField   `json:"params"`
	Returns         JSONReturnType `json:"returns"`
	AuthRequirement string         `json:"authRequirement"`
	Example         JSONExample    `json:"example"`
	ErrorCodes      []ErrorCode    `json:"errorCodes"`
}

// JSONReturnType 返回类型 JSON 结构
type JSONReturnType struct {
	SuccessType string `json:"successType"`
	HasData     bool   `json:"hasData"`
}

// JSONExample 示例 JSON 结构
type JSONExample struct {
	Request  interface{} `json:"request"`
	Response string      `json:"response,omitempty"`
}

// GenerateJSON 生成 JSON 文档
// changelog 参数可选，如果提供则使用它，否则不生成 changelog
func GenerateJSON(routerFiles []RouterFile, outputPath string, changelog *ChangelogEntry) error {
	// 获取元数据
	metadata := getMetadata()
	// 更新版本号（只在 JSON 文档中记录，不更新配置文件）
	metadata.Version = getGitVersion()

	// 构建 JSON 文档结构
	doc := JSONDocument{
		Metadata:    metadata,
		GeneratedAt: time.Now().Format("2006-01-02 15:04:05"),
		Info: JSONDocumentInfo{
			RequestFormat:  "{\"action\":\"<动作名称>\", \"params\": { ... }}",
			ResponseFormat: "JSON（包含 code/msg/data 等）",
		},
		Files: make([]JSONRouterFile, 0),
	}

	// 处理每个路由文件
	for _, rf := range routerFiles {
		if len(rf.Actions) == 0 {
			continue
		}

		fileTitle := getFileTitle(rf.FileName)
		jsonFile := JSONRouterFile{
			FileName:         rf.FileName,
			Title:            fileTitle,
			MiddlewareGroups: make([]JSONMiddlewareGroup, 0),
		}

		// 按中间件组合分组
		middlewareGroups := groupByMiddleware(rf.Actions)

		// 按中间件组排序
		var groupKeys []string
		for key := range middlewareGroups {
			groupKeys = append(groupKeys, key)
		}
		sort.Slice(groupKeys, func(i, j int) bool {
			hasAuthI := strings.Contains(groupKeys[i], "登录")
			hasAuthJ := strings.Contains(groupKeys[j], "登录")
			if hasAuthI != hasAuthJ {
				return !hasAuthI
			}
			return groupKeys[i] < groupKeys[j]
		})

		for _, groupKey := range groupKeys {
			actions := middlewareGroups[groupKey]
			if len(actions) == 0 {
				continue
			}

			jsonGroup := JSONMiddlewareGroup{
				Name:       groupKey,
				Categories: make([]JSONCategory, 0),
			}

			// 按功能分类分组
			categories := groupByCategory(actions)

			// 按分类排序
			var categoryKeys []string
			for key := range categories {
				categoryKeys = append(categoryKeys, key)
			}
			sort.Strings(categoryKeys)

			for _, categoryKey := range categoryKeys {
				categoryActions := categories[categoryKey]
				if len(categoryActions) == 0 {
					continue
				}

				jsonCategory := JSONCategory{
					Name:    categoryKey,
					Actions: make([]JSONAction, 0),
				}

				// 按 action 名称排序
				sort.Slice(categoryActions, func(i, j int) bool {
					return categoryActions[i].Name < categoryActions[j].Name
				})

				for _, action := range categoryActions {
					authReq := getAuthRequirement(action)

					// 构建示例（直接作为对象输出）
					// 将点号分隔的字段名转换为嵌套对象
					params := make(map[string]interface{})
					for _, p := range action.Params {
						if p.Required {
							// 检查参数名是否包含点号（如 "page.current"）
							if strings.Contains(p.Name, ".") {
								parts := strings.SplitN(p.Name, ".", 2)
								parentKey := parts[0]
								childKey := parts[1]

								// 如果父键不存在，创建嵌套对象
								if _, exists := params[parentKey]; !exists {
									params[parentKey] = make(map[string]interface{})
								}
								// 将子字段添加到嵌套对象中
								if parentObj, ok := params[parentKey].(map[string]interface{}); ok {
									parentObj[childKey] = getJSONExampleValue(p.Type)
								}
							} else {
								// 普通字段，直接添加
								params[p.Name] = getJSONExampleValue(p.Type)
							}
						}
					}

					exampleRequest := map[string]interface{}{
						"action": action.Name,
						"params": params,
					}

					jsonAction := JSONAction{
						Name:        action.Name,
						Description: action.Description,
						Params:      action.Params,
						Returns: JSONReturnType{
							SuccessType: action.Returns.SuccessType,
							HasData:     action.Returns.HasData,
						},
						AuthRequirement: authReq,
						Example: JSONExample{
							Request: exampleRequest,
						},
						ErrorCodes: action.Returns.ErrorCodes,
					}

					jsonCategory.Actions = append(jsonCategory.Actions, jsonAction)
				}

				jsonGroup.Categories = append(jsonGroup.Categories, jsonCategory)
			}

			jsonFile.MiddlewareGroups = append(jsonFile.MiddlewareGroups, jsonGroup)
		}

		doc.Files = append(doc.Files, jsonFile)
	}

	// 如果提供了 changelog，使用它
	if changelog != nil {
		doc.Changelog = changelog
	}

	// 生成 JSON
	jsonData, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("生成 JSON 失败: %w", err)
	}

	// 写入文件
	return os.WriteFile(outputPath, jsonData, 0644)
}

// GenerateGlobalChangelog 基于所有路由文件生成全局接口更新日志
// 通过对比当前版本和上一个版本的接口列表，识别新增和删除的接口
// snapshotDir 是快照文件保存的目录（通常是 docs 目录）
func GenerateGlobalChangelog(snapshotDir string, allRouterFiles []RouterFile) (*ChangelogEntry, error) {
	// 获取当前 commit 计数
	currentVersion := getGitCommitCount()
	if currentVersion == "-" {
		// 如果不在 git 仓库中，不生成更新日志
		return nil, nil
	}

	// 收集当前版本的所有接口名称
	currentActions := make(map[string]bool)
	for _, rf := range allRouterFiles {
		for _, action := range rf.Actions {
			currentActions[action.Name] = true
		}
	}

	// 转换为排序后的列表
	currentActionList := make([]string, 0, len(currentActions))
	for action := range currentActions {
		currentActionList = append(currentActionList, action)
	}
	sort.Strings(currentActionList)

	// 快照文件路径（在指定的目录中）
	snapshotPath := filepath.Join(snapshotDir, "api_snapshots.json")

	// 读取上一个版本的快照
	var previousSnapshot *ApiSnapshot
	if snapshotData, err := os.ReadFile(snapshotPath); err == nil {
		var snapshots []ApiSnapshot
		if err := json.Unmarshal(snapshotData, &snapshots); err == nil && len(snapshots) > 0 {
			// 获取最新的快照（最后一个）
			previousSnapshot = &snapshots[len(snapshots)-1]
		}
	}

	// 生成变更日志
	changelog := &ChangelogEntry{
		Version:   currentVersion,
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
		Added:     []string{},
		Removed:   []string{},
	}

	if previousSnapshot != nil {
		// 对比差异
		previousActions := make(map[string]bool)
		for _, action := range previousSnapshot.Actions {
			previousActions[action] = true
		}

		// 找出新增的接口
		for action := range currentActions {
			if !previousActions[action] {
				changelog.Added = append(changelog.Added, action)
			}
		}

		// 找出删除的接口
		for action := range previousActions {
			if !currentActions[action] {
				changelog.Removed = append(changelog.Removed, action)
			}
		}

		sort.Strings(changelog.Added)
		sort.Strings(changelog.Removed)
	} else {
		// 如果没有上一个版本，所有接口都标记为新增
		changelog.Added = currentActionList
	}

	// 如果没有变更，不生成日志条目
	if len(changelog.Added) == 0 && len(changelog.Removed) == 0 {
		return nil, nil
	}

	// 保存当前版本的快照
	currentSnapshot := ApiSnapshot{
		Version:   currentVersion,
		Timestamp: changelog.Timestamp,
		Actions:   currentActionList,
	}

	// 读取现有快照列表
	var snapshots []ApiSnapshot
	if snapshotData, err := os.ReadFile(snapshotPath); err == nil {
		json.Unmarshal(snapshotData, &snapshots)
	}

	// 检查是否已存在相同版本的快照
	found := false
	for i := range snapshots {
		if snapshots[i].Version == currentVersion {
			// 更新现有快照
			snapshots[i] = currentSnapshot
			found = true
			break
		}
	}

	if !found {
		// 添加新快照
		snapshots = append(snapshots, currentSnapshot)
	}

	// 保存快照列表（最多保留最近 50 个版本）
	if len(snapshots) > 50 {
		snapshots = snapshots[len(snapshots)-50:]
	}

	// 写入快照文件
	snapshotData, err := json.MarshalIndent(snapshots, "", "  ")
	if err == nil {
		os.WriteFile(snapshotPath, snapshotData, 0644)
	}

	return changelog, nil
}

// getJSONExampleValue 获取 JSON 示例值（返回 interface{}）
func getJSONExampleValue(typ string) interface{} {
	switch typ {
	case "int", "uint", "int32", "uint32", "int64", "uint64":
		return 0
	case "bool":
		return true
	case "[]string":
		return []string{}
	default:
		return ""
	}
}
