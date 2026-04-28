package mcp

import "errors"

var (
	ErrInvalidInputSchema = errors.New("mcp input schema must be an object schema")
	ErrInvalidArguments   = errors.New("mcp tool arguments must be a JSON object")
	ErrToolNotFound       = errors.New("mcp tool not found")
	ErrToolExists         = errors.New("mcp tool already exists")
	ErrMissingHandler     = errors.New("mcp tool handler is required")
)
