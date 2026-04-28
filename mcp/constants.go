package mcp

// https://modelcontextprotocol.io/specification/2025-11-25
const (
	DefaultProtocolVersion = "2025-11-25"
	DefaultServerName      = "aqi"
	DefaultServerVersion   = "0.1.0"
)

const (
	jsonrpcVersion = "2.0"

	methodInitialize               = "initialize"
	methodNotificationsInitialized = "notifications/initialized"
	methodPing                     = "ping"
	methodToolsList                = "tools/list"
	methodToolsCall                = "tools/call"

	contentTypeJSON = "application/json"
	contentTypeText = "text"

	defaultEmptyArguments = "{}"
)

const (
	rpcErrorParseError     = -32700
	rpcErrorInvalidRequest = -32600
	rpcErrorMethodNotFound = -32601
	rpcErrorInvalidParams  = -32602
)
