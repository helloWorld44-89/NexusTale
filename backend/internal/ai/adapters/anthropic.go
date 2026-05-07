package adapters

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	anthropicBaseURL = "https://api.anthropic.com/v1"
	anthropicVersion = "2023-06-01"
)

// Rough Anthropic token prices in USD per token.
var anthropicPricePerToken = map[string][2]float64{
	"claude-opus-4-6":              {0.000015, 0.000075},
	"claude-sonnet-4-6":            {0.000003, 0.000015},
	"claude-haiku-4-5-20251001":    {0.0000008, 0.000004},
	"claude-3-5-haiku-20241022":    {0.0000008, 0.000004},
	"claude-3-5-sonnet-20241022":   {0.000003, 0.000015},
}

// AnthropicAdapter calls the Anthropic Messages API.
// All Claude models support streaming — no thinking-model special-casing needed.
type AnthropicAdapter struct {
	apiKey string
	model  string
	client *http.Client
}

func NewAnthropicAdapter(apiKey, model string) *AnthropicAdapter {
	return &AnthropicAdapter{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

func (a *AnthropicAdapter) Provider() string      { return "anthropic" }
func (a *AnthropicAdapter) IsThinkingModel() bool { return false }

// ── request/response shapes ───────────────────────────────────────────────────

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
	Stream    bool               `json:"stream"`
}

type anthropicResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// anthropicErrorMessage parses the Anthropic error envelope:
//
//	{"type":"error","error":{"type":"overloaded_error","message":"..."}}
//
// Falls back to the raw body if parsing fails.
func anthropicErrorMessage(status int, body []byte) error {
	var envelope struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &envelope) == nil && envelope.Error.Message != "" {
		msg := envelope.Error.Message
		if status == 529 || envelope.Error.Type == "overloaded_error" {
			return fmt.Errorf("Anthropic is overloaded — please try again in a moment")
		}
		if status == 429 {
			return fmt.Errorf("Anthropic rate limit exceeded: %s", msg)
		}
		if status == 401 || status == 403 {
			return fmt.Errorf("Anthropic authentication error: %s", msg)
		}
		return fmt.Errorf("Anthropic error %d (%s): %s", status, envelope.Error.Type, msg)
	}
	return fmt.Errorf("anthropic %d: %s", status, string(body))
}

// ── helpers ───────────────────────────────────────────────────────────────────

func (a *AnthropicAdapter) estimateCost(input, output int) float64 {
	prices, ok := anthropicPricePerToken[a.model]
	if !ok {
		for k, v := range anthropicPricePerToken {
			if strings.HasPrefix(a.model, k) {
				prices = v
				ok = true
				break
			}
		}
	}
	if !ok {
		return 0
	}
	return float64(input)*prices[0] + float64(output)*prices[1]
}

func (a *AnthropicAdapter) post(ctx context.Context, body anthropicRequest) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		anthropicBaseURL+"/messages", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", a.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)
	req.Header.Set("Content-Type", "application/json")
	return a.client.Do(req)
}

// ── Complete (non-streaming) ──────────────────────────────────────────────────

func (a *AnthropicAdapter) Complete(ctx context.Context, req CompleteRequest) (string, Usage, error) {
	maxTok := req.MaxTokens
	if maxTok == 0 {
		maxTok = 1024
	}
	body := anthropicRequest{
		Model:     a.model,
		MaxTokens: maxTok,
		System:    req.SystemPrompt,
		Messages:  []anthropicMessage{{Role: "user", Content: req.Content}},
		Stream:    false,
	}

	resp, err := a.post(ctx, body)
	if err != nil {
		return "", Usage{}, fmt.Errorf("anthropic request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", Usage{}, anthropicErrorMessage(resp.StatusCode, b)
	}

	var result anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", Usage{}, fmt.Errorf("anthropic decode: %w", err)
	}
	if len(result.Content) == 0 {
		return "", Usage{}, fmt.Errorf("anthropic: empty response")
	}

	u := Usage{
		PromptTokens:     result.Usage.InputTokens,
		CompletionTokens: result.Usage.OutputTokens,
	}
	u.CostUSD = a.estimateCost(u.PromptTokens, u.CompletionTokens)
	return result.Content[0].Text, u, nil
}

// ── StreamComplete ────────────────────────────────────────────────────────────

func (a *AnthropicAdapter) StreamComplete(ctx context.Context, req CompleteRequest, w io.Writer) (Usage, error) {
	maxTok := req.MaxTokens
	if maxTok == 0 {
		maxTok = 1024
	}
	body := anthropicRequest{
		Model:     a.model,
		MaxTokens: maxTok,
		System:    req.SystemPrompt,
		Messages:  []anthropicMessage{{Role: "user", Content: req.Content}},
		Stream:    true,
	}

	resp, err := a.post(ctx, body)
	if err != nil {
		return Usage{}, fmt.Errorf("anthropic stream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return Usage{}, anthropicErrorMessage(resp.StatusCode, b)
	}

	return parseAnthropicStream(resp.Body, w)
}

// ── Chat / StreamChat ─────────────────────────────────────────────────────────

func (a *AnthropicAdapter) Chat(ctx context.Context, req ChatRequest) (string, Usage, error) {
	maxTok := req.MaxTokens
	if maxTok == 0 {
		maxTok = 1024
	}

	var system string
	msgs := []anthropicMessage{}
	for _, m := range req.Messages {
		if m.Role == "system" {
			system = m.Content
			continue
		}
		msgs = append(msgs, anthropicMessage{Role: m.Role, Content: m.Content})
	}

	body := anthropicRequest{
		Model:     a.model,
		MaxTokens: maxTok,
		System:    system,
		Messages:  msgs,
		Stream:    false,
	}

	resp, err := a.post(ctx, body)
	if err != nil {
		return "", Usage{}, fmt.Errorf("anthropic chat: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", Usage{}, anthropicErrorMessage(resp.StatusCode, b)
	}

	var result anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", Usage{}, fmt.Errorf("anthropic decode: %w", err)
	}
	if len(result.Content) == 0 {
		return "", Usage{}, fmt.Errorf("anthropic: empty response")
	}

	u := Usage{
		PromptTokens:     result.Usage.InputTokens,
		CompletionTokens: result.Usage.OutputTokens,
	}
	u.CostUSD = a.estimateCost(u.PromptTokens, u.CompletionTokens)
	return result.Content[0].Text, u, nil
}

func (a *AnthropicAdapter) StreamChat(ctx context.Context, req ChatRequest, w io.Writer) (Usage, error) {
	maxTok := req.MaxTokens
	if maxTok == 0 {
		maxTok = 1024
	}

	var system string
	msgs := []anthropicMessage{}
	for _, m := range req.Messages {
		if m.Role == "system" {
			system = m.Content
			continue
		}
		msgs = append(msgs, anthropicMessage{Role: m.Role, Content: m.Content})
	}

	body := anthropicRequest{
		Model:     a.model,
		MaxTokens: maxTok,
		System:    system,
		Messages:  msgs,
		Stream:    true,
	}

	resp, err := a.post(ctx, body)
	if err != nil {
		return Usage{}, fmt.Errorf("anthropic stream chat: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return Usage{}, anthropicErrorMessage(resp.StatusCode, b)
	}

	return parseAnthropicStream(resp.Body, w)
}

// ── Summarize ─────────────────────────────────────────────────────────────────

func (a *AnthropicAdapter) Summarize(ctx context.Context, text, systemPrompt string) (string, Usage, error) {
	req := CompleteRequest{
		SystemPrompt: systemPrompt,
		Content:      text,
		MaxTokens:    200,
	}
	return a.Complete(ctx, req)
}

// ── tool use ──────────────────────────────────────────────────────────────────

// anthropicTool is the wire format for a tool definition sent to the API.
type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// anthropicContentBlock represents one item in the content array of a response.
// When the model calls a tool, type == "tool_use". For normal text, type == "text".
type anthropicContentBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

// anthropicToolResponse is the non-streaming API response when tools are enabled.
type anthropicToolResponse struct {
	Content    []anthropicContentBlock `json:"content"`
	StopReason string                  `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// postJSON sends an arbitrary JSON body to the Anthropic messages endpoint.
// Used for tool-use requests that require a different message shape than post().
func (a *AnthropicAdapter) postJSON(ctx context.Context, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		anthropicBaseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", a.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)
	req.Header.Set("Content-Type", "application/json")
	return a.client.Do(req)
}

// ChatTools implements ToolAdapter. It runs one non-streaming tool-use round using
// Anthropic's tools API and returns the model's response (tool calls or final text).
func (a *AnthropicAdapter) ChatTools(ctx context.Context, msgs []Message, extraMsgs []json.RawMessage, tools []ToolDefinition, maxTokens int) (ToolChatResponse, error) {
	if maxTokens == 0 {
		maxTokens = 1024
	}

	// Separate system message; convert the rest to raw JSON (string content).
	var system string
	rawMsgs := make([]json.RawMessage, 0, len(msgs)+len(extraMsgs))
	for _, m := range msgs {
		if m.Role == "system" {
			system = m.Content
			continue
		}
		raw, _ := json.Marshal(map[string]string{"role": m.Role, "content": m.Content})
		rawMsgs = append(rawMsgs, raw)
	}
	rawMsgs = append(rawMsgs, extraMsgs...)

	anthropicTools := make([]anthropicTool, len(tools))
	for i, t := range tools {
		anthropicTools[i] = anthropicTool{Name: t.Name, Description: t.Description, InputSchema: t.InputSchema}
	}

	reqMap := map[string]interface{}{
		"model":      a.model,
		"max_tokens": maxTokens,
		"messages":   rawMsgs,
		"tools":      anthropicTools,
		"stream":     false,
	}
	if system != "" {
		reqMap["system"] = system
	}

	data, err := json.Marshal(reqMap)
	if err != nil {
		return ToolChatResponse{}, err
	}

	resp, err := a.postJSON(ctx, data)
	if err != nil {
		return ToolChatResponse{}, fmt.Errorf("anthropic tools: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return ToolChatResponse{}, anthropicErrorMessage(resp.StatusCode, b)
	}

	var result anthropicToolResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ToolChatResponse{}, fmt.Errorf("anthropic tools decode: %w", err)
	}

	out := ToolChatResponse{
		StopReason: result.StopReason,
		Usage: Usage{
			PromptTokens:     result.Usage.InputTokens,
			CompletionTokens: result.Usage.OutputTokens,
		},
	}
	out.Usage.CostUSD = a.estimateCost(out.Usage.PromptTokens, out.Usage.CompletionTokens)

	for _, block := range result.Content {
		switch block.Type {
		case "text":
			out.Text += block.Text
		case "tool_use":
			out.ToolCalls = append(out.ToolCalls, ToolCall{
				ID:    block.ID,
				Name:  block.Name,
				Input: block.Input,
			})
		}
	}

	// Preserve the full assistant message (with all content blocks) for replay.
	assistantMsg := map[string]interface{}{
		"role":    "assistant",
		"content": result.Content,
	}
	out.RawAssistantMsg, _ = json.Marshal(assistantMsg)

	return out, nil
}

// BuildToolResultMessages converts ToolResults into the Anthropic tool-result
// user message (a single user turn with a content array of tool_result blocks).
func (a *AnthropicAdapter) BuildToolResultMessages(results []ToolResult) []json.RawMessage {
	blocks := make([]map[string]interface{}, len(results))
	for i, r := range results {
		block := map[string]interface{}{
			"type":        "tool_result",
			"tool_use_id": r.ID,
			"content":     r.Content,
		}
		if r.IsError {
			block["is_error"] = true
		}
		blocks[i] = block
	}
	msg := map[string]interface{}{"role": "user", "content": blocks}
	raw, _ := json.Marshal(msg)
	return []json.RawMessage{raw}
}

// ── SSE parser ────────────────────────────────────────────────────────────────

type anthropicStreamEvent struct {
	Type  string `json:"type"`
	Delta *struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta"`
	Message *struct {
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	} `json:"message"`
	Usage *struct {
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// parseAnthropicStream reads Anthropic SSE events and writes NexusTale SSE format.
// Anthropic emits: event: content_block_delta / data: {...}
func parseAnthropicStream(body io.Reader, w io.Writer) (Usage, error) {
	var u Usage
	scanner := bufio.NewScanner(body)

	for scanner.Scan() {
		line := scanner.Text()

		// Skip event: lines; only process data: lines.
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")

		var evt anthropicStreamEvent
		if err := json.Unmarshal([]byte(payload), &evt); err != nil {
			continue
		}

		switch evt.Type {
		case "message_start":
			if evt.Message != nil {
				u.PromptTokens = evt.Message.Usage.InputTokens
			}
		case "content_block_delta":
			if evt.Delta != nil && evt.Delta.Type == "text_delta" && evt.Delta.Text != "" {
				encoded, _ := json.Marshal(map[string]string{"delta": evt.Delta.Text})
				fmt.Fprintf(w, "data: %s\n\n", encoded)
			}
		case "message_delta":
			if evt.Usage != nil {
				u.CompletionTokens = evt.Usage.OutputTokens
			}
		case "message_stop":
			fmt.Fprintf(w, "data: [DONE]\n\n")
		}
	}

	return u, scanner.Err()
}
