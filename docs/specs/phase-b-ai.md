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

// CompleteMode controls how the AI expands the input.
//   - "continue" (default): append a continuation of the current scene content.
//   - "beat": treat Content as a story beat (1-sentence intent) and expand it
//     into 2–3 paragraphs of prose. Uses a beat-specific system prompt prefix.
type CompleteMode string

const (
    CompleteModeContinue CompleteMode = "continue"
    CompleteModeBeat     CompleteMode = "beat"
)

type CompleteRequest struct {
    SystemPrompt string
    Content      string       // current scene text (continue) or beat sentence (beat)
    Mode         CompleteMode // defaults to CompleteModeContinue
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
    // For thinking models (o1, o3, deepseek-reasoner, qwq, r1) the adapter
    // automatically falls back to Complete and simulates streaming.
    StreamComplete(ctx context.Context, req CompleteRequest, w io.Writer) (Usage, error)

    // StreamChat writes SSE chunks to the provided writer.
    StreamChat(ctx context.Context, req ChatRequest, w io.Writer) (Usage, error)

    // Summarize condenses text to a short paragraph (non-streaming).
    Summarize(ctx context.Context, text string) (string, Usage, error)

    // IsThinkingModel returns true when the configured model uses chain-of-thought
    // (o1, o3, deepseek-reasoner, qwq, r1). Callers should skip system prompt
    // injection and prefer batch over streaming for these models.
    IsThinkingModel() bool

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
func NewAdapter(provider string, apiKey string, model string, cfg Config) (Adapter, error)
```
- Called per-request with the decrypted key from `internal/auth.DecryptAPIKey`
- Falls back to Ollama if `provider == "ollama"` or key is empty
- Returns error if provider is unknown and no Ollama configured

#### Thinking model detection
```go
var thinkingModelSubstrings = []string{"o1", "o3", "deepseek-reasoner", "qwq", "r1"}

func isThinkingModel(modelID string) bool {
    lower := strings.ToLower(modelID)
    for _, sub := range thinkingModelSubstrings {
        if strings.Contains(lower, sub) {
            return true
        }
    }
    return false
}
```
When `IsThinkingModel()` returns true the adapter:
1. Skips injecting a system-role message (these models don't accept them in standard position)
2. Falls back from `StreamComplete` → `Complete` and simulates streaming by splitting the
   response on whitespace and writing `data: {"delta":"<word>"}` lines with a 10ms delay

The `forceNonStreaming` field on `user_api_keys` (added in B1.5) overrides streaming for any
model, useful when a provider claims streaming support but behaves unreliably.

---

## HTTP routes

All routes require `RequireAuth`. The handler calls `DecryptAPIKey` to get the provider + key for the requesting user, constructs an adapter, calls the service.

```
POST /projects/:id/ai/complete    — scene continuation or beat expansion (streaming SSE)
POST /projects/:id/ai/chat        — freeform chat (streaming SSE)
POST /projects/:id/ai/summarize   — scene → summary (JSON)
GET  /projects/:id/ai/usage       — token/cost aggregate (JSON)
```

### Request bodies

```json
// complete — continue mode (default)
{ "scene_id": "uuid", "mode": "continue", "instruction": "continue the scene" }

// complete — beat mode
// AI expands the beat into 2–3 paragraphs of prose using the beat system prompt.
// The scene's stored tense and pov are injected into the system prompt template.
{ "scene_id": "uuid", "mode": "beat", "beat": "Jack discovers the hidden door is already ajar" }

// complete — with writing style override (B1.5)
{ "scene_id": "uuid", "mode": "beat", "beat": "...", "prompt_id": "uuid" }

// chat
{ "messages": [{"role": "user", "content": "..."}], "scene_id": "uuid" }

// summarize
{ "scene_id": "uuid" }
```

### Beat mode system prompt template

When `mode=beat` and no `prompt_id` is provided the backend uses this default:

```
You are a co-author helping write a {genre} novel called "{title}".
Write in {tense} tense from {pov} point of view. The POV character is {pov_character}.
Given a story beat (what should happen next), write 2–3 paragraphs that bring the beat to life.
Match the author's tone and style. Use sensory details. Show, don't tell.
```

Placeholders resolved from: `projects.genres` (comma-joined), `projects.title`,
`scenes.tense`, `scenes.pov`, `scenes.pov_character`. Missing values are omitted gracefully.

### Streaming response (SSE)
```
data: {"delta": "Once upon"}
data: {"delta": " a time"}
data: [DONE]
```

---

## Context window (B2)

`internal/ai/context.go` — `BuildContext(ctx, db, projectID, sceneID, userText string) ([]Message, error)`

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

4. Inline @[entity] mentions (resolved from userText or current scene content)
   role: "system", content: "Referenced entity '{name}' ({type}): {summary}"
   — parsed before step 4; injected immediately before the user message

5. Current scene content (for /complete) or user message (for /chat)
```

Token budget: 8000 tokens total. If over budget, truncate oldest chapter summaries first, then older scenes.

### Inline `@[entity]` resolution

`internal/ai/context.go` parses `userText` (the beat, instruction, or chat message) for
`@[Entity Name]` patterns before building the context. Each mention is resolved against
`wiki_entities` by exact name match (case-insensitive). Resolved entries are injected as
`role: "system"` messages immediately before the final user message, deduplicated by ID.

The 10-entity cap from step 1 is separate — inline mentions are additive (up to 5 additional
entries) and counted against the token budget after chapter summaries.

---

---

## Writing Styles / Prose Prompts (B1.5)

Writers can create named prompt presets that override the system prompt and/or inject a
user-facing style instruction at generation time. This lets a writer switch between
"gritty cyberpunk noir" and "high fantasy epic voice" without touching any settings.

### Migration 014

```sql
CREATE TABLE project_prompts (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id     UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name           TEXT NOT NULL,
    category       TEXT NOT NULL DEFAULT 'prose',  -- 'prose' | 'workshop'
    content        TEXT NOT NULL DEFAULT '',        -- user-facing style instruction (appended to user turn)
    system_content TEXT NOT NULL DEFAULT '',        -- system prompt override (replaces default when set)
    sort_order     INT  NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### Routes

```
GET    /projects/:id/prompts           → list all writing styles for project
POST   /projects/:id/prompts           → create
PUT    /projects/:id/prompts/:promptId → update
DELETE /projects/:id/prompts/:promptId → delete
```

### How prompts are applied

When `prompt_id` is provided in a `/ai/complete` or `/ai/chat` request:
1. `system_content` (if non-empty) replaces the default system prompt entirely.
   Template placeholders (`{title}`, `{genre}`, `{tense}`, `{pov}`, `{pov_character}`) are
   still substituted from the project/scene metadata.
2. `content` (if non-empty) is appended to the final user message as a style instruction block:
   ```
   ---
   Style guidance: {content}
   ```
3. If `system_content` is empty but `content` is set, the default system prompt is used and
   only the style guidance block is added.

### `forceNonStreaming` field

Add `force_non_streaming BOOL NOT NULL DEFAULT false` to `user_api_keys` (alter in migration 014).
When true, `StreamComplete`/`StreamChat` fall back to batch + simulated streaming regardless of
model. Useful for providers that nominally support SSE but behave unreliably.

### Frontend — Style selector

- Dropdown in `SceneMetadataPanel` below tense/POV: "Writing style: Default (NexusTale)"
- Selecting a style stores `selectedPromptId` in local component state; sent with every AI call
- "Manage styles" link → `/projects/:id/prompts` (simple CRUD list page)
- Empty state encourages the writer to create their first style with an example

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
- `@[Entity Name]` inline mentions are resolved server-side — no special client parsing needed

### SceneMetadataPanel
- Show `ai_summary_stale` badge ("Summary outdated") when true
- "Regenerate" button → `POST /ai/summarize` → updates `scene.summary` locally
- Writing style dropdown: list from `GET /projects/:id/prompts`; selected `prompt_id` sent with every AI call
- Beat input field below the writing style selector; send with `mode: "beat"` when non-empty

### Beat UX (ScribeEditor toolbar)
- Small "Beat →" button in the editor toolbar (or bottom of ScribeEditor)
- Clicking opens an inline input below the editor textarea
- Writer types a beat sentence; pressing Enter (or clicking "Generate") sends `POST /ai/complete`
  with `mode: "beat"`, `beat: <text>`, `scene_id`, `prompt_id`
- Streaming response is appended to the scene content with a highlight (5-second fade)
- Accept / Retry / Discard actions appear after generation completes

---

## OpenAPI schemas (additions)

```yaml
AICompleteRequest:
  type: object
  required: [scene_id]
  properties:
    scene_id:  { type: string, format: uuid }
    mode:      { type: string, enum: [continue, beat], default: continue }
    beat:      { type: string, description: "Required when mode=beat" }
    instruction: { type: string, description: "Optional hint for continue mode" }
    prompt_id: { type: string, format: uuid, description: "Writing style preset to apply" }

ProjectPromptResponse:
  type: object
  required: [id, name, category, content, system_content, sort_order]
  properties:
    id:             { type: string, format: uuid }
    name:           { type: string }
    category:       { type: string, enum: [prose, workshop] }
    content:        { type: string }
    system_content: { type: string }
    sort_order:     { type: integer }
```

---

## Environment variables

```env
NEXUSTALE_OLLAMA_URL=http://localhost:11434   # fallback local AI
NEXUSTALE_OLLAMA_MODEL=llama3.2
NEXUSTALE_AI_MAX_TOKENS=2048                 # default max tokens per call
NEXUSTALE_AI_CONTEXT_TOKENS=8000            # context window budget
NEXUSTALE_AI_BEAT_MAX_TOKENS=600            # cap for beat expansion (2–3 paragraphs)
NEXUSTALE_AI_INLINE_ENTITY_LIMIT=5         # max @[entity] mentions resolved per request
```
