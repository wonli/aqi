package mcp

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/wonli/aqi"
	"github.com/wonli/aqi/logger"
)

type AuthFunc func(r *http.Request) bool

type Option func(*Server)

type Server struct {
	app             *aqi.AppConfig
	protocolVersion string
	name            string
	version         string

	auth AuthFunc

	mu          sync.RWMutex
	initialized bool
	tools       map[string]registeredTool
}

func NewServer(app *aqi.AppConfig, options ...Option) *Server {
	s := &Server{
		app:             app,
		protocolVersion: DefaultProtocolVersion,
		name:            DefaultServerName,
		version:         DefaultServerVersion,
		tools:           map[string]registeredTool{},
	}

	for _, option := range options {
		if option != nil {
			option(s)
		}
	}

	return s
}

func WithProtocolVersion(version string) Option {
	return func(s *Server) {
		if version != "" {
			s.protocolVersion = version
		}
	}
}

func WithName(name string) Option {
	return func(s *Server) {
		if name != "" {
			s.name = name
		}
	}
}

func WithVersion(version string) Option {
	return func(s *Server) {
		if version != "" {
			s.version = version
		}
	}
}

func WithBearerToken(token string) Option {
	return func(s *Server) {
		if token == "" {
			return
		}

		s.auth = func(r *http.Request) bool {
			value := r.Header.Get("Authorization")
			if !strings.HasPrefix(value, "Bearer ") {
				return false
			}

			got := strings.TrimSpace(strings.TrimPrefix(value, "Bearer "))
			return subtle.ConstantTimeCompare([]byte(got), []byte(token)) == 1
		}
	}
}

func WithAuth(fn AuthFunc) Option {
	return func(s *Server) {
		s.auth = fn
	}
}

func (s *Server) Tool(name string, tool Tool) {
	if err := s.registerTool(name, tool); err != nil {
		panic(err)
	}
}

func (s *Server) registerTool(name string, tool Tool) error {
	if name == "" {
		return fmt.Errorf("mcp tool name is required")
	}
	if tool.Handler == nil {
		return ErrMissingHandler
	}

	tool.InputSchema = normalizeSchema(tool.InputSchema)
	if err := validateInputSchema(tool.InputSchema); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.tools[name]; ok {
		return ErrToolExists
	}

	s.tools[name] = registeredTool{name: name, Tool: tool}
	return nil
}

func (s *Server) HTTPHandler() http.Handler {
	return http.HandlerFunc(s.serveHTTP)
}

func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "mcp server supports POST only", http.StatusMethodNotAllowed)
		return
	}

	if s.auth != nil && !s.auth(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req rpcRequest
	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()
	if err := decoder.Decode(&req); err != nil {
		writeRPC(w, rpcResponse{
			JSONRPC: jsonrpcVersion,
			Error:   &rpcError{Code: rpcErrorParseError, Message: "parse error", Data: err.Error()},
		})
		return
	}

	if req.JSONRPC != jsonrpcVersion || req.Method == "" {
		writeRPC(w, rpcResponse{
			JSONRPC: jsonrpcVersion,
			ID:      req.ID,
			Error:   &rpcError{Code: rpcErrorInvalidRequest, Message: "invalid request"},
		})
		return
	}

	result, rpcErr, notification, status := s.dispatch(r, req)
	if notification {
		w.WriteHeader(status)
		return
	}

	res := rpcResponse{JSONRPC: jsonrpcVersion, ID: req.ID}
	if rpcErr != nil {
		res.Error = rpcErr
	} else {
		res.Result = result
	}

	writeRPC(w, res)
}

func (s *Server) dispatch(r *http.Request, req rpcRequest) (any, *rpcError, bool, int) {
	notification := len(req.ID) == 0

	switch req.Method {
	case methodInitialize:
		return s.initialize(), nil, notification, http.StatusAccepted
	case methodNotificationsInitialized:
		s.mu.Lock()
		s.initialized = true
		s.mu.Unlock()
		return nil, nil, true, http.StatusAccepted
	case methodPing:
		return emptyResult{}, nil, notification, http.StatusAccepted
	case methodToolsList:
		return s.toolsList(), nil, notification, http.StatusAccepted
	case methodToolsCall:
		result, err := s.toolsCall(r, req.Params)
		if err != nil {
			return nil, err, notification, http.StatusAccepted
		}

		return result, nil, notification, http.StatusAccepted
	default:
		return nil, &rpcError{Code: rpcErrorMethodNotFound, Message: "method not found"}, notification, http.StatusAccepted
	}
}

func (s *Server) initialize() initializeResult {
	return initializeResult{
		ProtocolVersion: s.protocolVersion,
		Capabilities:    capabilities{Tools: toolsCapability{ListChanged: false}},
		ServerInfo:      serverInfo{Name: s.name, Version: s.version},
	}
}

func (s *Server) toolsList() toolsListResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tools := make([]toolInfo, 0, len(s.tools))
	for name, tool := range s.tools {
		tools = append(tools, toolInfo{
			Name:        name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
			Annotations: tool.annotations(),
		})
	}

	return toolsListResult{Tools: tools}
}

func (s *Server) toolsCall(r *http.Request, params json.RawMessage) (any, *rpcError) {
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &rpcError{Code: rpcErrorInvalidParams, Message: "invalid params", Data: err.Error()}
	}
	if p.Name == "" {
		return nil, &rpcError{Code: rpcErrorInvalidParams, Message: "tool name is required"}
	}
	if len(p.Arguments) == 0 || string(p.Arguments) == "null" {
		p.Arguments = []byte(defaultEmptyArguments)
	}

	s.mu.RLock()
	tool, ok := s.tools[p.Name]
	s.mu.RUnlock()
	if !ok {
		return nil, &rpcError{Code: rpcErrorInvalidParams, Message: ErrToolNotFound.Error()}
	}

	if err := validateArguments(tool.InputSchema, p.Arguments); err != nil {
		return nil, &rpcError{Code: rpcErrorInvalidParams, Message: "invalid arguments", Data: err.Error()}
	}

	start := time.Now()
	result := s.callTool(r, tool, p.Arguments)
	logInfof("mcp tool=%s duration=%s error=%t", p.Name, time.Since(start), result.IsError)

	return result, nil
}

func (s *Server) callTool(r *http.Request, tool registeredTool, args json.RawMessage) toolResult {
	ctx := r.Context()
	cancel := func() {}
	if tool.Policy.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, tool.Policy.Timeout)
	}
	defer cancel()

	done := make(chan callResult, 1)
	mcpCtx := newContext(ctx, r, tool.name, args)
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				done <- callResult{ctx: mcpCtx, err: fmt.Errorf("mcp tool panic: %v", recovered)}
			}
		}()

		tool.Handler(mcpCtx)
		done <- callResult{ctx: mcpCtx}
	}()

	select {
	case <-ctx.Done():
		return errorToolResult(ctx.Err())
	case res := <-done:
		if res.err != nil {
			return errorToolResult(res.err)
		}

		data, msg, isErr := res.ctx.responseData()
		if isErr {
			return toolResult{
				Content:           []contentBlock{{Type: contentTypeText, Text: textSummary(data, msg)}},
				StructuredContent: data,
				IsError:           true,
			}
		}

		return successToolResult(data, msg)
	}
}

func validateArguments(schema Schema, arguments json.RawMessage) error {
	var args map[string]json.RawMessage
	if err := json.Unmarshal(arguments, &args); err != nil {
		return ErrInvalidArguments
	}
	if args == nil {
		return ErrInvalidArguments
	}

	for _, name := range schema.Required {
		if _, ok := args[name]; !ok {
			return fmt.Errorf("missing required argument %q", name)
		}
	}

	for name, prop := range schema.Properties {
		value, ok := args[name]
		if !ok {
			continue
		}

		if err := validateArgumentType(name, prop, value); err != nil {
			return err
		}
	}

	return nil
}

func validateArgumentType(name string, schema Schema, value any) error {
	if schema.Type == "" || value == nil {
		return nil
	}

	raw, ok := value.(json.RawMessage)
	if !ok {
		return nil
	}

	switch schema.Type {
	case "string":
		var v string
		if err := json.Unmarshal(raw, &v); err != nil {
			return fmt.Errorf("argument %q must be a string", name)
		}
	case "number":
		var v json.Number
		if err := json.Unmarshal(raw, &v); err != nil {
			return fmt.Errorf("argument %q must be a number", name)
		}
		if _, err := v.Float64(); err != nil {
			return fmt.Errorf("argument %q must be a number", name)
		}
	case "integer":
		var v json.Number
		if err := json.Unmarshal(raw, &v); err != nil {
			return fmt.Errorf("argument %q must be an integer", name)
		}
		if _, err := v.Int64(); err != nil {
			return fmt.Errorf("argument %q must be an integer", name)
		}
	case "boolean":
		var v bool
		if err := json.Unmarshal(raw, &v); err != nil {
			return fmt.Errorf("argument %q must be a boolean", name)
		}
	case "object":
		var v map[string]json.RawMessage
		if err := json.Unmarshal(raw, &v); err != nil {
			return fmt.Errorf("argument %q must be an object", name)
		}
	case "array":
		var v []json.RawMessage
		if err := json.Unmarshal(raw, &v); err != nil {
			return fmt.Errorf("argument %q must be an array", name)
		}
	}

	return nil
}

func successToolResult(data any, msg string) toolResult {
	return toolResult{
		Content:           []contentBlock{{Type: contentTypeText, Text: textSummary(data, msg)}},
		StructuredContent: data,
	}
}

func errorToolResult(err error) toolResult {
	if err == nil {
		err = errors.New("unknown error")
	}

	return toolResult{
		Content: []contentBlock{{
			Type: contentTypeText,
			Text: err.Error(),
		}},
		IsError: true,
	}
}

func writeRPC(w http.ResponseWriter, res rpcResponse) {
	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(res)
}

type callResult struct {
	ctx *Context
	err error
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id,omitempty"`
	Result  any       `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e rpcError) IsZero() bool {
	return e.Code == 0 && e.Message == "" && e.Data == nil
}

type initializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    capabilities `json:"capabilities"`
	ServerInfo      serverInfo   `json:"serverInfo"`
}

type emptyResult struct{}

type capabilities struct {
	Tools toolsCapability `json:"tools"`
}

type toolsCapability struct {
	ListChanged bool `json:"listChanged"`
}

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type toolsListResult struct {
	Tools []toolInfo `json:"tools"`
}

type toolInfo struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema Schema          `json:"inputSchema"`
	Annotations ToolAnnotations `json:"annotations,omitempty"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type toolResult struct {
	Content           []contentBlock `json:"content"`
	StructuredContent any            `json:"structuredContent,omitempty"`
	IsError           bool           `json:"isError,omitempty"`
}

func logInfof(template string, args ...any) {
	if logger.SugarLog == nil {
		return
	}

	logger.SugarLog.Infof(template, args...)
}
