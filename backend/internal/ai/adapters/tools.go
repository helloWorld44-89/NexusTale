package adapters

import (
	"context"
	"encoding/json"
)

// ToolDefinition describes a callable tool the AI may invoke.
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"` // JSON Schema (type:"object")
}

// ToolCall is a single invocation request returned by the model in a tool-use round.
type ToolCall struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// ToolResult is the caller's output for one tool call, ready to send back to the model.
type ToolResult struct {
	ID      string `json:"id"`      // matches ToolCall.ID
	Content string `json:"content"` // human-readable result text
	IsError bool   `json:"is_error,omitempty"`
}

// ToolChatResponse is the parsed outcome of one non-streaming tool-use round.
//
// StopReason values:
//
//	"tool_use" — ToolCalls is populated; execute them and loop.
//	"end_turn" — Text is the final reply; stop the loop.
type ToolChatResponse struct {
	StopReason string
	Text       string
	ToolCalls  []ToolCall
	Usage      Usage
	// RawAssistantMsg is the opaque JSON of the full assistant message for this
	// round. Callers must append it to the conversation before sending tool results.
	RawAssistantMsg json.RawMessage
}

// ToolAdapter is an optional extension of Adapter for providers that support
// structured tool / function calling.
//
// Check for it with a type assertion:
//
//	if ta, ok := adapter.(ToolAdapter); ok { ... }
//
// Ollama does not implement this interface; callers should fall back to StreamChat.
type ToolAdapter interface {
	// ChatTools performs one non-streaming tool-use round.
	// extraMsgs is an opaque slice of raw JSON messages from prior rounds
	// (assistant tool-use turn + user tool-result turn) appended after msgs.
	ChatTools(ctx context.Context, msgs []Message, extraMsgs []json.RawMessage, tools []ToolDefinition, maxTokens int) (ToolChatResponse, error)

	// BuildToolResultMessages converts a batch of ToolResults into the raw JSON
	// messages to append to extraMsgs before the next round.
	BuildToolResultMessages(results []ToolResult) []json.RawMessage
}
