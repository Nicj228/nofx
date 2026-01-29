package assistant

import (
	"context"
	"encoding/json"
)

// Tool represents a callable tool that the AI agent can use
type Tool interface {
	// Name returns the tool's unique identifier
	Name() string

	// Description returns a human-readable description for the AI
	Description() string

	// ParameterSchema returns JSON schema for the tool's parameters
	ParameterSchema() string

	// Execute runs the tool with the given arguments
	Execute(ctx context.Context, args json.RawMessage) (interface{}, error)
}

// BaseTool provides common functionality for tools
type BaseTool struct {
	ToolName        string
	ToolDescription string
	ToolSchema      string
	ExecuteFunc     func(ctx context.Context, args json.RawMessage) (interface{}, error)
}

func (t *BaseTool) Name() string        { return t.ToolName }
func (t *BaseTool) Description() string { return t.ToolDescription }
func (t *BaseTool) ParameterSchema() string { return t.ToolSchema }

func (t *BaseTool) Execute(ctx context.Context, args json.RawMessage) (interface{}, error) {
	return t.ExecuteFunc(ctx, args)
}

// NewTool creates a simple tool from a function
func NewTool(name, description, schema string, fn func(ctx context.Context, args json.RawMessage) (interface{}, error)) Tool {
	return &BaseTool{
		ToolName:        name,
		ToolDescription: description,
		ToolSchema:      schema,
		ExecuteFunc:     fn,
	}
}
