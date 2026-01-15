package docgen

// RouterFile 路由文件信息
type RouterFile struct {
	FileName string      // 路由文件名（如 "action.go"）
	FuncName string      // 路由注册函数名（如 "Actions", "AdminAction"）
	Actions  []ActionDoc // 该文件中的所有 action
}

// ActionDoc Action 文档结构
type ActionDoc struct {
	Name            string       // action 名称
	Description     string       // 功能描述（从函数注释提取）
	RouterFile      string       // 所属路由文件
	MiddlewareChain []string     // 中间件链（如 ["Recovery", "App", "Auth"]）
	MiddlewareGroup string       // 中间件组变量名（"r", "app", "auth", "adm", "admLog" 等）
	Params          []ParamField // 请求参数列表
	Returns         ReturnType   // 返回类型
	Examples        []Example    // 使用示例
}

// ParamField 参数字段
type ParamField struct {
	Name        string // 参数名
	Type        string // 类型
	Required    bool   // 是否必需
	Description string // 描述
}

// ErrorCode 错误码信息
type ErrorCode struct {
	Code    int    // 错误码
	Message string // 错误消息
}

// ReturnType 返回类型
type ReturnType struct {
	SuccessType string      // 成功返回类型
	ErrorCodes  []ErrorCode // 错误码列表
	HasData     bool        // 是否有数据返回
}

// Example 使用示例
type Example struct {
	Request  string // 请求示例
	Response string // 响应示例
}

// MiddlewareInfo 中间件信息
type MiddlewareInfo struct {
	VarName         string   // 变量名（如 "app", "auth"）
	MiddlewareChain []string // 中间件链
	ParentVar       string   // 父变量名（如 "app" 的父变量是 "r"）
}
