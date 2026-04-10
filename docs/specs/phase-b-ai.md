# Phase B — AI Sub-spec: Adapters, Routes & Memory

## Overview

This spec covers B1 (AI proxy), B2 (AI memory/context), and B3 (token tracking). These three are tightly coupled — B2 feeds context into B1's routes, B3 records the result.

---

## Adapter interface

`internal/ai/adapter.go`

```go
type Message struct {
    Role    string // "system" | "user" | "assistant"
    Content string
}

type CompleteRequest struct {
    SystemPrompt string
    Content      string   // current scene text
    MaxTokens    int
}

type ChatRequest struct {
    Messages  []Message
    MaxTokens int
}

type Adapter interface {
    // Complete returns a scene continuation (non-streaming).
    Complete(ctx context.Context, req CompleteRequest) (string, Usage, error)

    // Chat returns an assistant reply (non-streaming).
    Chat(ctx context.Context, req ChatRequest) (string, Usage, error)

    // StreamComplete writes SSE chunks to the provided writer.
    StreamComplete(ctx context.Context, req CompleteRequest, w io.Writer) (Usage, error)

    // StreamChat writes SSE chunks to the provided writer.
    StreamChat(ctx context.Context, req ChatRequest, w io.Writer) (Usage, error)

    // Summarize condenses text to a short paragraph (non-streaming).
    Summarize(ctx context.Context, text string) (string, Usage, error)

    // Provider returns the canonical provider name ("openai", "anthropic", "ollama").
    Provider() string
}

type Usage struct {
    PromptTokens     int
    CompletionTokens int
    CostUSD          float64
}
```

---

## Provider implementations

### OpenAI (`internal/ai/openai.go`)
- Base URL: `https://api.openai.com/v1`
- Default model: `gpt-4o-mini` (cheap, fast); configurable per-request
- Auth: `Authorization: Bearer <key>`
- Streaming: `stream: true` → parse `data: {...}` SSE lines
- Cost estimate: use published token prices; configurable multiplier for future-proofing

### Anthropic (`internal/ai/anthropic.go`)
- Base URL: `https://api.anthropic.com/v1`
- Default model: `claude-haiku-4-5-20251001`
- Auth: `x-api-key: <key>` + `anthropic-version: 2023-06-01`
- Streaming: `event: content_block_delta` SSE events
- Messages API (not completions)

### Ollama (`internal/ai/ollama.go`)
- Base URL: `http://localhost:11434` (configurable via `NEXUSTALE_OLLAMA_URL`)
- No auth needed
- Model: from `NEXUSTALE_OLLAMA_MODEL` env (default `llama3.2`)
- Used as fallback when no cloud key stored; also used in dev/testing

### AdapterFactory (`internal/ai/factory.go`)
```go
func NewAdapter(provider string, apiKey string, cfg Config) (Adapter, error)
```
- Called per-request with the decrypted key from `internal/auth.DecryptAPIKey`
- Falls back to Ollama if `provider == "ollama"` or key is empty
- Returns error if provider is unknown and no Ollama configured

---

## HTTP routes

All routes require `RequireAuth`. The handler calls `DecryptAPIKey` to get the provider + key for the requesting user, constructs an adapter, calls the service.

```
POST /projects/:id/ai/complete    — scene continuation (streaming SSE)
POST /projects/:id/ai/chat        — freeform chat (streaming SSE)
POST /projects/:id/ai/summarize   — scene → summary (JSON)
GET  /projects/:id/ai/usage       — token/cost aggregate (JSON)
```

### Request bodies

```json
// complete
{ "scene_id": "uuid", "instruction": "continue the scene" }

// chat
{ "messages": [{"role": "user", "content": "..."}], "scene_id": "uuid" }

// summarize
{ "scene_id": "uuid" }
```

### Streaming response (SSE)
```
data: {"delta": "Once upon"}
data: {"delta": " a time"}
data: [DONE]
```

---

## Context window (B2)

`internal/ai/context.go` — `BuildContext(ctx, db, projectID, sceneID) ([]Message, error)`

Assembly order (innermost = most recent = highest priority):

```
1. System message
   "You are a writing assistant for a {genre} novel called '{title}'.
    World context: {wiki entity summaries, comma-joined, max 10 entities}
    Writing style: {tense} {POV}"

2. Chapter summary messages (last 3 chapters, oldest first)
   role: "system", content: "Chapter '{title}' summary: {ai_summary}"

3. Recent scene content (last 2 scenes before current, oldest first)
   role: "user" (simulate what was written), content: "{scene content, truncated to 1500 tokens}"

4. Current scene content (for /complete) or user message (for /chat)
```

Token budget: 8000 tokens total. If over budget, truncate oldest chapter summaries first, then older scenes.

---

## Auto-summarize goroutine (B2)

`internal/ai/summarizer.go`

- Triggered by the scene update handler after a successful save
- Uses a per-chapter debounce: 30s window, coalesces multiple rapid saves
- Goroutine calls `Summarize(scene contents of chapter, joined)`
- On success: `UPDATE chapters SET ai_summary = $1, ai_summary_stale = false WHERE id = $2`
- On failure: logs error, leaves `ai_summary_stale = true`
- Does not block the HTTP response path

---

## Token tracking (B3)

`internal/ai/usage.go`

After every adapter call, record non-blocking:
```go
go func() {
    _ = queries.InsertUsage(ctx, sqlcgen.InsertUsageParams{
        UserID:           userID,
        ProjectID:        projectID,
        Model:            model,
        PromptTokens:     usage.PromptTokens,
        CompletionTokens: usage.CompletionTokens,
        CostUsd:          usage.CostUSD,
    })
}()
```

`GET /projects/:id/ai/usage` returns:
```json
{
  "total_tokens": 142000,
  "total_cost_usd": 0.43,
  "monthly_tokens": 28000,
  "monthly_cost_usd": 0.08,
  "calls_this_month": 47
}
```

---

## Frontend wiring

### ChatBar (`src/components/ai/ChatBar.tsx`)
- Currently renders a static placeholder
- Wire to `POST /projects/:id/ai/chat` with SSE streaming
- Message list: user bubbles (right) + assistant bubbles (left)
- Typing indicator while streaming
- Scene context: send current `scene_id` so backend builds context window

### SceneMetadataPanel
- Show `ai_summary_stale` badge ("Summary outdated") when true
- "Regenerate" button → `POST /ai/summarize` → updates `scene.summary` locally

---

## Environment variables

```env
NEXUSTALE_OLLAMA_URL=http://localhost:11434   # fallback local AI
NEXUSTALE_OLLAMA_MODEL=llama3.2
NEXUSTALE_AI_MAX_TOKENS=2048                 # default max tokens per call
NEXUSTALE_AI_CONTEXT_TOKENS=8000            # context window budget
```
