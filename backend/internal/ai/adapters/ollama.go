package adapters

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OllamaAdapter speaks to a local Ollama server (default http://localhost:11434).
// It uses the /api/chat endpoint with the OpenAI-compatible message format.
// No API key is required.
type OllamaAdapter struct {
	baseURL string
	model   string
	client  *http.Client
}

func NewOllamaAdapter(baseURL, model string) *OllamaAdapter {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if model == "" {
		model = "llama3.2"
	}
	return &OllamaAdapter{
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{Timeout: 180 * time.Second},
	}
}

func (a *OllamaAdapter) Provider() string      { return "ollama" }
func (a *OllamaAdapter) IsThinkingModel() bool { return false }

// ── request/response shapes ───────────────────────────────────────────────────

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  *ollamaOptions  `json:"options,omitempty"`
}

type ollamaOptions struct {
	NumPredict int `json:"num_predict,omitempty"` // max tokens
}

type ollamaStreamChunk struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Done               bool `json:"done"`
	PromptEvalCount    int  `json:"prompt_eval_count"`
	EvalCount          int  `json:"eval_count"`
}

// ── helpers ───────────────────────────────────────────────────────────────────

func (a *OllamaAdapter) buildMessages(req CompleteRequest) []ollamaMessage {
	msgs := []ollamaMessage{}
	if req.SystemPrompt != "" {
		msgs = append(msgs, ollamaMessage{Role: "system", Content: req.SystemPrompt})
	}
	msgs = append(msgs, ollamaMessage{Role: "user", Content: req.Content})
	return msgs
}

func (a *OllamaAdapter) post(ctx context.Context, body ollamaRequest) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		a.baseURL+"/api/chat", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return a.client.Do(req)
}

// ── Complete (non-streaming) ──────────────────────────────────────────────────

func (a *OllamaAdapter) Complete(ctx context.Context, req CompleteRequest) (string, Usage, error) {
	body := ollamaRequest{
		Model:    a.model,
		Messages: a.buildMessages(req),
		Stream:   false,
	}
	if req.MaxTokens > 0 {
		body.Options = &ollamaOptions{NumPredict: req.MaxTokens}
	}

	resp, err := a.post(ctx, body)
	if err != nil {
		return "", Usage{}, fmt.Errorf("ollama request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", Usage{}, fmt.Errorf("ollama %d: %s", resp.StatusCode, string(b))
	}

	// Non-streaming Ollama returns a single JSON object (not NDJSON).
	var chunk ollamaStreamChunk
	if err := json.NewDecoder(resp.Body).Decode(&chunk); err != nil {
		return "", Usage{}, fmt.Errorf("ollama decode: %w", err)
	}

	u := Usage{
		PromptTokens:     chunk.PromptEvalCount,
		CompletionTokens: chunk.EvalCount,
		CostUSD:          0, // local — no cost
	}
	return chunk.Message.Content, u, nil
}

// ── StreamComplete ────────────────────────────────────────────────────────────

func (a *OllamaAdapter) StreamComplete(ctx context.Context, req CompleteRequest, w io.Writer) (Usage, error) {
	body := ollamaRequest{
		Model:    a.model,
		Messages: a.buildMessages(req),
		Stream:   true,
	}
	if req.MaxTokens > 0 {
		body.Options = &ollamaOptions{NumPredict: req.MaxTokens}
	}

	resp, err := a.post(ctx, body)
	if err != nil {
		return Usage{}, fmt.Errorf("ollama stream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return Usage{}, fmt.Errorf("ollama %d: %s", resp.StatusCode, string(b))
	}

	return parseOllamaStream(resp.Body, w)
}

// ── Chat / StreamChat ─────────────────────────────────────────────────────────

func (a *OllamaAdapter) Chat(ctx context.Context, req ChatRequest) (string, Usage, error) {
	msgs := make([]ollamaMessage, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = ollamaMessage{Role: m.Role, Content: m.Content}
	}
	body := ollamaRequest{
		Model:    a.model,
		Messages: msgs,
		Stream:   false,
	}
	if req.MaxTokens > 0 {
		body.Options = &ollamaOptions{NumPredict: req.MaxTokens}
	}

	resp, err := a.post(ctx, body)
	if err != nil {
		return "", Usage{}, fmt.Errorf("ollama chat: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", Usage{}, fmt.Errorf("ollama %d: %s", resp.StatusCode, string(b))
	}

	var chunk ollamaStreamChunk
	if err := json.NewDecoder(resp.Body).Decode(&chunk); err != nil {
		return "", Usage{}, fmt.Errorf("ollama decode: %w", err)
	}

	return chunk.Message.Content, Usage{
		PromptTokens:     chunk.PromptEvalCount,
		CompletionTokens: chunk.EvalCount,
	}, nil
}

func (a *OllamaAdapter) StreamChat(ctx context.Context, req ChatRequest, w io.Writer) (Usage, error) {
	msgs := make([]ollamaMessage, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = ollamaMessage{Role: m.Role, Content: m.Content}
	}
	body := ollamaRequest{
		Model:    a.model,
		Messages: msgs,
		Stream:   true,
	}
	if req.MaxTokens > 0 {
		body.Options = &ollamaOptions{NumPredict: req.MaxTokens}
	}

	resp, err := a.post(ctx, body)
	if err != nil {
		return Usage{}, fmt.Errorf("ollama stream chat: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return Usage{}, fmt.Errorf("ollama %d: %s", resp.StatusCode, string(b))
	}

	return parseOllamaStream(resp.Body, w)
}

// ── Summarize ─────────────────────────────────────────────────────────────────

func (a *OllamaAdapter) Summarize(ctx context.Context, text, systemPrompt string) (string, Usage, error) {
	req := CompleteRequest{
		SystemPrompt: systemPrompt,
		Content:      text,
		MaxTokens:    200,
	}
	return a.Complete(ctx, req)
}

// ── NDJSON stream parser ──────────────────────────────────────────────────────

// parseOllamaStream reads Ollama's newline-delimited JSON stream and writes
// NexusTale SSE format.
func parseOllamaStream(body io.Reader, w io.Writer) (Usage, error) {
	var u Usage
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		var chunk ollamaStreamChunk
		if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
			continue
		}

		if chunk.Message.Content != "" {
			encoded, _ := json.Marshal(map[string]string{"delta": chunk.Message.Content})
			fmt.Fprintf(w, "data: %s\n\n", encoded)
		}

		if chunk.Done {
			u.PromptTokens = chunk.PromptEvalCount
			u.CompletionTokens = chunk.EvalCount
			fmt.Fprintf(w, "data: [DONE]\n\n")
			break
		}
	}
	return u, scanner.Err()
}
