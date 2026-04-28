package mcp

import "time"

type HandlerFunc func(ctx *Context)

type Tool struct {
	Description string
	InputSchema Schema
	Policy      ToolPolicy
	Annotations ToolAnnotations
	Handler     HandlerFunc
}

type ToolPolicy struct {
	ReadOnly             bool
	Destructive          bool
	RequiresConfirmation bool
	Timeout              time.Duration
}

type ToolAnnotations struct {
	Title                string `json:"title,omitempty"`
	ReadOnlyHint         bool   `json:"readOnlyHint"`
	DestructiveHint      bool   `json:"destructiveHint"`
	RequiresConfirmation bool   `json:"requiresConfirmation,omitempty"`
}

type registeredTool struct {
	name string
	Tool
}

func (t Tool) annotations() ToolAnnotations {
	annotations := t.Annotations
	if !annotations.ReadOnlyHint {
		annotations.ReadOnlyHint = t.Policy.ReadOnly
	}
	if !annotations.DestructiveHint {
		annotations.DestructiveHint = t.Policy.Destructive
	}
	if !annotations.RequiresConfirmation {
		annotations.RequiresConfirmation = t.Policy.RequiresConfirmation
	}

	return annotations
}
