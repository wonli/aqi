package mcp

import (
	"bytes"
	"encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestInitialize(t *testing.T) {
	server := NewServer(nil, WithName("test-aqi"), WithVersion("1.2.3"))
	body, status := postRPC(t, server, "", rpcRequestBody{JSONRPC: jsonrpcVersion, ID: 1, Method: methodInitialize})
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, DefaultProtocolVersion, gjson.Get(body, "result.protocolVersion").String())
	require.Equal(t, "test-aqi", gjson.Get(body, "result.serverInfo.name").String())
	require.Equal(t, "1.2.3", gjson.Get(body, "result.serverInfo.version").String())
	require.True(t, gjson.Get(body, "result.capabilities.tools").Exists())
}

func TestInitializeWithProtocolVersion(t *testing.T) {
	server := NewServer(nil, WithProtocolVersion("custom-version"))
	body, status := postRPC(t, server, "", rpcRequestBody{JSONRPC: jsonrpcVersion, ID: 1, Method: methodInitialize})
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, "custom-version", gjson.Get(body, "result.protocolVersion").String())
}

func TestInitializeWithEmptyProtocolVersionKeepsDefault(t *testing.T) {
	server := NewServer(nil, WithProtocolVersion(""))
	body, status := postRPC(t, server, "", rpcRequestBody{JSONRPC: jsonrpcVersion, ID: 1, Method: methodInitialize})
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, DefaultProtocolVersion, gjson.Get(body, "result.protocolVersion").String())
}

func TestHTTPRejectsNonPost(t *testing.T) {
	server := NewServer(nil)
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	rec := httptest.NewRecorder()
	server.HTTPHandler().ServeHTTP(rec, req)
	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	require.Equal(t, http.MethodPost, rec.Header().Get("Allow"))
}

func TestParseError(t *testing.T) {
	server := NewServer(nil)
	body, status := postRawRPC(t, server, "", `{"jsonrpc":"2.0","id":1,`)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, int64(rpcErrorParseError), gjson.Get(body, "error.code").Int())
	require.Equal(t, "parse error", gjson.Get(body, "error.message").String())
}

func TestInvalidRequest(t *testing.T) {
	server := NewServer(nil)
	body, status := postRawRPC(t, server, "", `{"jsonrpc":"1.0","id":7,"method":"ping"}`)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, int64(rpcErrorInvalidRequest), gjson.Get(body, "error.code").Int())
	require.Equal(t, int64(7), gjson.Get(body, "id").Int())
}

func TestMethodNotFound(t *testing.T) {
	server := NewServer(nil)
	body, status := postRPC(t, server, "", rpcRequestBody{JSONRPC: jsonrpcVersion, ID: 1, Method: "does/not/exist"})
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, int64(rpcErrorMethodNotFound), gjson.Get(body, "error.code").Int())
}

func TestNotificationHasNoResponseBody(t *testing.T) {
	server := NewServer(nil)
	body, status := postRPC(t, server, "", rpcRequestBody{JSONRPC: jsonrpcVersion, Method: methodPing})
	require.Equal(t, http.StatusAccepted, status)
	require.Empty(t, body)
}

func TestInitializedNotificationSetsState(t *testing.T) {
	server := NewServer(nil)
	body, status := postRPC(t, server, "", rpcRequestBody{JSONRPC: jsonrpcVersion, Method: methodNotificationsInitialized})
	require.Equal(t, http.StatusAccepted, status)
	require.Empty(t, body)
	server.mu.RLock()
	initialized := server.initialized
	server.mu.RUnlock()
	require.True(t, initialized)
}

func TestToolsList(t *testing.T) {
	server := NewServer(nil)
	server.Tool("weather.query", Tool{Description: "Query weather.", InputSchema: ObjectSchema(map[string]Schema{"city": StringSchema("City name")}, "city"), Policy: ToolPolicy{ReadOnly: true}, Handler: func(ctx *Context) {}})
	body, status := postRPC(t, server, "", rpcRequestBody{JSONRPC: jsonrpcVersion, ID: 1, Method: methodToolsList})
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, "weather.query", gjson.Get(body, "result.tools.0.name").String())
	require.Equal(t, "Query weather.", gjson.Get(body, "result.tools.0.description").String())
	require.Equal(t, "object", gjson.Get(body, "result.tools.0.inputSchema.type").String())
	require.True(t, gjson.Get(body, "result.tools.0.annotations.readOnlyHint").Bool())
}

func TestToolsCallReturnValue(t *testing.T) {
	server := NewServer(nil)
	server.Tool("weather.query", Tool{InputSchema: ObjectSchema(map[string]Schema{"city": StringSchema("City name")}, "city"), Handler: func(ctx *Context) {
		var req weatherRequest
		require.NoError(t, ctx.Bind(&req))
		ctx.Send(weatherResponse{City: req.City, Weather: "sunny"})
	}})
	body, status := postRPC(t, server, "", rpcRequestBody{JSONRPC: jsonrpcVersion, ID: "call-1", Method: methodToolsCall, Params: toolCallParams{Name: "weather.query", Arguments: weatherRequest{City: "Shanghai"}}})
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, "Shanghai", gjson.Get(body, "result.structuredContent.city").String())
	require.Equal(t, "sunny", gjson.Get(body, "result.structuredContent.weather").String())
	require.False(t, gjson.Get(body, "result.isError").Bool())
}

func TestToolsCallDefaultsMissingArgumentsToObject(t *testing.T) {
	server := NewServer(nil)
	server.Tool("empty", Tool{InputSchema: EmptyObjectSchema(), Handler: func(ctx *Context) {
		var args map[string]any
		require.NoError(t, ctx.Bind(&args))
		require.NotNil(t, args)
		require.Empty(t, args)
		ctx.SendOk()
	}})
	body, status := postRPC(t, server, "", rpcRequestBody{JSONRPC: jsonrpcVersion, ID: 1, Method: methodToolsCall, Params: toolCallParams{Name: "empty"}})
	require.Equal(t, http.StatusOK, status)
	require.False(t, gjson.Get(body, "result.isError").Bool())
}

func TestToolsCallSendCodeReturnsToolError(t *testing.T) {
	server := NewServer(nil)
	server.Tool("fail", Tool{InputSchema: EmptyObjectSchema(), Handler: func(ctx *Context) { ctx.SendCode(4001, "city is required") }})
	body, status := postRPC(t, server, "", rpcRequestBody{JSONRPC: jsonrpcVersion, ID: 1, Method: methodToolsCall, Params: toolCallParams{Name: "fail"}})
	require.Equal(t, http.StatusOK, status)
	require.True(t, gjson.Get(body, "result.isError").Bool())
	require.Equal(t, int64(4001), gjson.Get(body, "result.structuredContent.code").Int())
}

func TestToolsCallInvalidArguments(t *testing.T) {
	server := NewServer(nil)
	called := false
	server.Tool("weather.query", Tool{InputSchema: ObjectSchema(map[string]Schema{"city": StringSchema("City name")}, "city"), Handler: func(ctx *Context) { called = true }})
	body, status := postRPC(t, server, "", rpcRequestBody{JSONRPC: jsonrpcVersion, ID: 1, Method: methodToolsCall, Params: toolCallParams{Name: "weather.query", Arguments: map[string]any{"city": 12}}})
	require.Equal(t, http.StatusOK, status)
	require.False(t, called)
	require.Equal(t, int64(rpcErrorInvalidParams), gjson.Get(body, "error.code").Int())
	require.Contains(t, gjson.Get(body, "error.data").String(), "must be a string")
}

func TestToolsCallRequiredArgument(t *testing.T) {
	server := NewServer(nil)
	server.Tool("weather.query", Tool{InputSchema: ObjectSchema(map[string]Schema{"city": StringSchema("")}, "city"), Handler: func(ctx *Context) { t.Fatal("handler must not run") }})
	body, _ := postRPC(t, server, "", rpcRequestBody{JSONRPC: jsonrpcVersion, ID: 1, Method: methodToolsCall, Params: toolCallParams{Name: "weather.query", Arguments: map[string]any{}}})
	require.Equal(t, int64(rpcErrorInvalidParams), gjson.Get(body, "error.code").Int())
	require.Contains(t, gjson.Get(body, "error.data").String(), "missing required argument")
}

func TestArgumentTypeValidation(t *testing.T) {
	tests := []struct {
		name string
		schema Schema
		value string
		wantErr bool
	}{
		{"string ok", StringSchema(""), `"x"`, false}, {"string bad", StringSchema(""), `1`, true},
		{"number integer", NumberSchema(""), `1`, false}, {"number exponent", NumberSchema(""), `1e3`, false}, {"number bad", NumberSchema(""), `"1"`, true},
		{"integer ok", IntegerSchema(""), `42`, false}, {"integer fraction", IntegerSchema(""), `1.5`, true}, {"integer exponent", IntegerSchema(""), `1e3`, true},
		{"boolean ok", BooleanSchema(""), `true`, false}, {"boolean bad", BooleanSchema(""), `"true"`, true},
		{"object ok", Schema{Type: "object"}, `{}`, false}, {"object bad", Schema{Type: "object"}, `[]`, true},
		{"array ok", ArraySchema(StringSchema(""), ""), `[]`, false}, {"array bad", ArraySchema(StringSchema(""), ""), `{}`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateArgumentType("v", tt.schema, []byte(tt.value))
			if tt.wantErr { require.Error(t, err) } else { require.NoError(t, err) }
		})
	}
}

func TestToolRegistrationValidation(t *testing.T) {
	server := NewServer(nil)
	require.Error(t, server.registerTool("", Tool{Handler: func(ctx *Context) {}}))
	require.ErrorIs(t, server.registerTool("missing.handler", Tool{}), ErrMissingHandler)
	require.ErrorIs(t, server.registerTool("bad.schema", Tool{InputSchema: StringSchema(""), Handler: func(ctx *Context) {}}), ErrInvalidInputSchema)
	require.NoError(t, server.registerTool("ok", Tool{Handler: func(ctx *Context) {}}))
	require.ErrorIs(t, server.registerTool("ok", Tool{Handler: func(ctx *Context) {}}), ErrToolExists)
}

func TestToolRegistrationPanic(t *testing.T) {
	server := NewServer(nil)
	require.PanicsWithError(t, ErrMissingHandler.Error(), func() { server.Tool("missing.handler", Tool{InputSchema: EmptyObjectSchema()}) })
}

func TestMissingTool(t *testing.T) {
	server := NewServer(nil)
	body, status := postRPC(t, server, "", rpcRequestBody{JSONRPC: jsonrpcVersion, ID: 1, Method: methodToolsCall, Params: toolCallParams{Name: "missing"}})
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, int64(rpcErrorInvalidParams), gjson.Get(body, "error.code").Int())
	require.Equal(t, ErrToolNotFound.Error(), gjson.Get(body, "error.message").String())
}

func TestHandlerError(t *testing.T) {
	server := NewServer(nil)
	server.Tool("fail", Tool{InputSchema: EmptyObjectSchema(), Handler: func(ctx *Context) { ctx.Error(errors.New("boom")) }})
	body, status := postRPC(t, server, "", rpcRequestBody{JSONRPC: jsonrpcVersion, ID: 1, Method: methodToolsCall, Params: toolCallParams{Name: "fail"}})
	require.Equal(t, http.StatusOK, status)
	require.True(t, gjson.Get(body, "result.isError").Bool())
	require.Equal(t, "boom", gjson.Get(body, "result.content.0.text").String())
}

func TestBearerAuth(t *testing.T) {
	server := NewServer(nil, WithBearerToken("secret"))
	_, status := postRPC(t, server, "", rpcRequestBody{JSONRPC: jsonrpcVersion, ID: 1, Method: methodInitialize})
	require.Equal(t, http.StatusUnauthorized, status)
	_, status = postRPC(t, server, "Bearer wrong", rpcRequestBody{JSONRPC: jsonrpcVersion, ID: 1, Method: methodInitialize})
	require.Equal(t, http.StatusUnauthorized, status)
	_, status = postRPC(t, server, "Bearer secret", rpcRequestBody{JSONRPC: jsonrpcVersion, ID: 1, Method: methodInitialize})
	require.Equal(t, http.StatusOK, status)
}

func TestCustomAuth(t *testing.T) {
	server := NewServer(nil, WithAuth(func(r *http.Request) bool { return r.Header.Get("X-Test-Auth") == "yes" }))
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	req.Header.Set("X-Test-Auth", "yes")
	rec := httptest.NewRecorder()
	server.HTTPHandler().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestTimeout(t *testing.T) {
	server := NewServer(nil)
	server.Tool("slow", Tool{InputSchema: EmptyObjectSchema(), Policy: ToolPolicy{Timeout: time.Millisecond}, Handler: func(ctx *Context) { time.Sleep(20 * time.Millisecond); ctx.Send("done") }})
	body, status := postRPC(t, server, "", rpcRequestBody{JSONRPC: jsonrpcVersion, ID: 1, Method: methodToolsCall, Params: toolCallParams{Name: "slow"}})
	require.Equal(t, http.StatusOK, status)
	require.True(t, gjson.Get(body, "result.isError").Bool())
	require.Contains(t, gjson.Get(body, "result.content.0.text").String(), "deadline")
}

func TestHandlerPanic(t *testing.T) {
	server := NewServer(nil)
	server.Tool("panic", Tool{InputSchema: EmptyObjectSchema(), Handler: func(ctx *Context) { panic("bad day") }})
	body, status := postRPC(t, server, "", rpcRequestBody{JSONRPC: jsonrpcVersion, ID: 1, Method: methodToolsCall, Params: toolCallParams{Name: "panic"}})
	require.Equal(t, http.StatusOK, status)
	require.True(t, gjson.Get(body, "result.isError").Bool())
	require.Contains(t, gjson.Get(body, "result.content.0.text").String(), "panic")
}

func postRPC(t *testing.T, server *Server, auth string, req rpcRequestBody) (string, int) {
	t.Helper()
	data, err := json.Marshal(req)
	require.NoError(t, err)
	return postRawRPC(t, server, auth, string(data))
}

func postRawRPC(t *testing.T, server *Server, auth string, data string) (string, int) {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(data))
	request.Header.Set("Content-Type", contentTypeJSON)
	if auth != "" { request.Header.Set("Authorization", auth) }
	recorder := httptest.NewRecorder()
	server.HTTPHandler().ServeHTTP(recorder, request)
	return recorder.Body.String(), recorder.Code
}

type rpcRequestBody struct { JSONRPC string `json:"jsonrpc"`; ID any `json:"id,omitempty"`; Method string `json:"method"`; Params any `json:"params,omitempty"` }
type toolCallParams struct { Name string `json:"name"`; Arguments any `json:"arguments,omitempty"` }
type weatherRequest struct { City string `json:"city"` }
type weatherResponse struct { City string `json:"city"`; Weather string `json:"weather"` }
