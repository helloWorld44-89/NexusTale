package adapters

import (
	"context"
	"io"
)

// CompleteMode controls how the AI processes the input.
//
//   - Continue: append a natural continuation of the current scene content.
//   - Beat: treat Content as a single-sentence story intent and expand it
//     into 2–3 paragraphs of prose using the beat system prompt.
type CompleteMode string

const (
	CompleteModeContinue CompleteMode = "continue"
	CompleteModeBeat     CompleteMode = "beat"
)

// Message is a single turn in a chat conversation.
type Message struct {
	Role    string `json:"role"`    // "system" | "user" | "assistant"
	Content string `json:"content"`
}

// CompleteRequest carries everything the adapter needs for a completion call.
type CompleteRequest struct {
	SystemPrompt string
	Content      string       // scene text (continue) or beat sentence (beat)
	Mode         CompleteMode // defaults to CompleteModeContinue
	MaxTokens    int
}

// ChatRequest carries a full conversation history for chat mode.
type ChatRequest struct {
	Messages  []Message
	MaxTokens int
}

// Usage records token consumption and estimated cost for one AI call.
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	CostUSD          float64
}

// Adapter is the common interface all AI backends must satisfy.
//
// Streaming methods write NexusTale SSE format to w:
//
//	data: {"delta":"word "}\n\n
//	data: [DONE]\n\n
//
// Thinking models (o1, o3, deepseek-reasoner, qwq, r1) must be detected by
// IsThinkingModel; callers should skip system prompt injection and prefer the
// non-streaming Complete/Chat methods for those models.
type Adapter interface {
	// Complete returns the full response text (non-streaming).
	Complete(ctx context.Context, req CompleteRequest) (string, Usage, error)

	// Chat returns a single assistant reply (non-streaming).
	Chat(ctx context.Context, req ChatRequest) (string, Usage, error)

	// StreamComplete writes SSE chunks to w. For thinking models it falls back
	// to Complete internally and simulates streaming.
	StreamComplete(ctx context.Context, req CompleteRequest, w io.Writer) (Usage, error)

	// StreamChat writes SSE chunks to w.
	StreamChat(ctx context.Context, req ChatRequest, w io.Writer) (Usage, error)

	// Summarize condenses text to a short paragraph (non-streaming).
	// systemPrompt is caller-supplied so the service layer can inject genre context.
	Summarize(ctx context.Context, text, systemPrompt string) (string, Usage, error)

	// IsThinkingModel returns true when the configured model uses chain-of-thought
	// reasoning and does not support standard system prompts or streaming.
	IsThinkingModel() bool

	// Provider returns the canonical provider name ("openai", "anthropic", "ollama").
	Provider() string
}
