package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/spf13/cast"
	"github.com/tidwall/gjson"
)

type Response struct {
	Code int    `json:"code,omitempty"`
	Msg  string `json:"msg,omitempty"`
	Data any    `json:"data,omitempty"`
}

type response struct {
	code int
	msg  string
	data any
	err  error
	set  bool
}

// Context is the per-call context passed to MCP tool handlers.
//
// It intentionally mirrors the most common ws.Context conveniences: handlers can
// bind JSON arguments, read single fields with Get helpers, or call Send-style
// methods.
type Context struct {
	context.Context

	ToolName  string
	Request   *http.Request
	Arguments json.RawMessage

	response response
}

func newContext(ctx context.Context, r *http.Request, toolName string, args json.RawMessage) *Context {
	return &Context{
		Context:   ctx,
		ToolName:  toolName,
		Request:   r,
		Arguments: args,
	}
}

func (c *Context) Bind(v any) error {
	return json.Unmarshal(c.argumentsOrEmpty(), v)
}

func (c *Context) BindingJson(v any) error {
	return c.Bind(v)
}

func (c *Context) GetJson(v any) error {
	return c.Bind(v)
}

func (c *Context) Get(key string) string {
	return gjson.GetBytes(c.argumentsOrEmpty(), key).String()
}

func (c *Context) GetInt(key string) int {
	return int(gjson.GetBytes(c.argumentsOrEmpty(), key).Int())
}

func (c *Context) GetBool(key string) bool {
	return gjson.GetBytes(c.argumentsOrEmpty(), key).Bool()
}

func (c *Context) GetId(key string) uint {
	v := c.GetInt(key)
	if v <= 0 {
		return 0
	}

	return uint(v)
}

func (c *Context) GetSliceVal(key string, options ...string) string {
	v := c.Get(key)
	for _, option := range options {
		if v == option {
			return v
		}
	}

	return ""
}

func (c *Context) GetMinInt(key string, min int) int {
	v := c.GetInt(key)
	if v < min {
		return min
	}

	return v
}

func (c *Context) GetRangeInt(key string, min, max int) int {
	v := c.GetMinInt(key, min)
	if v > max {
		return max
	}

	return v
}

func (c *Context) Send(data any) {
	c.response = response{data: data, set: true}
}

func (c *Context) SendOk() {
	c.response = response{set: true}
}

func (c *Context) SendMsg(msg string) {
	c.response = response{msg: msg, set: true}
}

func (c *Context) SendCode(code int, msg string) {
	c.response = response{code: code, msg: msg, set: true}
}

func (c *Context) Error(err error) {
	if err == nil {
		return
	}

	c.response = response{err: err, set: true}
}

func (c *Context) ErrorString(msg string) {
	c.Error(errors.New(msg))
}

func (c *Context) Header(key string) string {
	if c.Request == nil {
		return ""
	}

	return c.Request.Header.Get(key)
}

func (c *Context) Caller() string {
	if c.Request == nil {
		return ""
	}

	if forwarded := c.Request.Header.Get("X-Forwarded-For"); forwarded != "" {
		return forwarded
	}

	return c.Request.RemoteAddr
}

func (c *Context) responseData() (any, string, bool) {
	if c.response.set {
		if c.response.err != nil {
			return nil, c.response.err.Error(), true
		}
		if c.response.code != 0 {
			return Response{Code: c.response.code, Msg: c.response.msg, Data: c.response.data}, c.response.msg, true
		}
		if c.response.msg != "" {
			return Response{Msg: c.response.msg, Data: c.response.data}, c.response.msg, false
		}

		return c.response.data, "", false
	}

	return nil, "", false
}

func (c *Context) argumentsOrEmpty() []byte {
	if len(c.Arguments) == 0 || string(c.Arguments) == "null" {
		return []byte(defaultEmptyArguments)
	}

	return c.Arguments
}

func textSummary(data any, fallback string) string {
	if fallback != "" {
		return fallback
	}

	if data == nil {
		return "ok"
	}

	switch v := data.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return cast.ToString(data)
	}
}
