package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const openRouterBaseURL = "https://openrouter.ai/api/v1"

// Rough token prices for common OpenRouter-routed models (USD per token).
// OpenRouter adds a small markup on top of provider costs; these are estimates.
var openRouterPricePerToken = map[string][2]float64{
	"openai/gpt-4o":                    {0.0000025, 0.000010},
	"openai/gpt-4o-mini":               {0.00000015, 0.0000006},
	"anthropic/claude-3-5-sonnet":      {0.000003, 0.000015},
	"anthropic/claude-3-5-haiku":       {0.00000025, 0.00000125},
	"anthropic/claude-3-haiku":         {0.00000025, 0.00000125},
	"google/gemini-flash-1.5":          {0.000000075, 0.0000003},
	"google/gemini-pro-1.5":            {0.00000125, 0.000005},
	"meta-llama/llama-3.1-8b-instruct": {0.000000055, 0.000000055},
	"mistralai/mistral-7b-instruct":    {0.000000055, 0.000000055},
}

// OpenRouterAdapter calls the OpenRouter unified API (OpenAI wire format).
// OpenRouter routes requests to the underlying provider; the model field uses
// the "provider/model-name" notation (e.g. "anthropic/claude-3-5-haiku").
type OpenRouterAdapter struct {
	apiKey   string
	model    string
	thinking bool
	client   *http.Client
}

func NewOpenRouterAdapter(apiKey, model string) *OpenRouterAdapter {
	if model == "" {
		model = "anthropic/claude-3-5-haiku"
	}
	return &OpenRouterAdapter{
		apiKey:   apiKey,
		model:    model,
		thinking: isThinkingModel(model),
		client:   &http.Client{Timeout: 120 * time.Second},
	}
}

func (a *OpenRouterAdapter) Provider() string      { return "openrouter" }
func (a *OpenRouterAdapter) IsThinkingModel() bool { return a.thinking }

// ── helpers ───────────────────────────────────────────────────────────────────

func (a *OpenRouterAdapter) buildMessages(req CompleteRequest) []openAIMessage {
	msgs := []openAIMessage{}
	if req.SystemPrompt != "" && !a.thinking {
		msgs = append(msgs, openAIMessage{Role: "system", Content: req.SystemPrompt})
	}
	msgs = append(msgs, openAIMessage{Role: "user", Content: req.Content})
	return msgs
}

func (a *OpenRouterAdapter) buildChatMessages(req ChatRequest) []openAIMessage {
	msgs := make([]openAIMessage, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = openAIMessage{Role: m.Role, Content: m.Content}
	}
	return msgs
}

func (a *OpenRouterAdapter) estimateCost(prompt, completion int) float64 {
	prices, ok := openRouterPricePerToken[a.model]
	if !ok {
		for k, v := range openRouterPricePerToken {
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
	return float64(prompt)*prices[0] + float64(completion)*prices[1]
}

// post sends an openAIRequest body with the required OpenRouter headers.
func (a *OpenRouterAdapter) post(ctx context.Context, body openAIRequest) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return a.doRequest(ctx, data)
}

// postJSON sends an arbitrary JSON body (used for tool-calling requests).
func (a *OpenRouterAdapter) postJSON(ctx context.Context, body []byte) (*http.Response, error) {
	return a.doRequest(ctx, body)
}

func (a *OpenRouterAdapter) doRequest(ctx context.Context, data []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		openRouterBaseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", "https://nexustale.app")
	req.Header.Set("X-Title", "NexusTale")
	return a.client.Do(req)
}

// ── Complete ──────────────────────────────────────────────────────────────────

func (a *OpenRouterAdapter) Complete(ctx context.Context, req CompleteRequest) (string, Usage, error) {
	body := openAIRequest{
		Model:    a.model,
		Messages: a.buildMessages(req),
		Stream:   false,
	}
	if a.thinking {
		body.MaxCompletionTokens = req.MaxTokens
	} else {
		body.MaxTokens = req.MaxTokens
	}

	resp, err := a.post(ctx, body)
	if err != nil {
		return "", Usage{}, fmt.Errorf("openrouter request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", Usage{}, fmt.Errorf("openrouter %d: %s", resp.StatusCode, string(b))
	}

	var result openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", Usage{}, fmt.Errorf("openrouter decode: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", Usage{}, fmt.Errorf("openrouter: no choices returned")
	}

	u := Usage{
		PromptTokens:     result.Usage.PromptTokens,
		CompletionTokens: result.Usage.CompletionTokens,
	}
	u.CostUSD = a.estimateCost(u.PromptTokens, u.CompletionTokens)
	return result.Choices[0].Message.Content, u, nil
}

// ── StreamComplete ────────────────────────────────────────────────────────────

func (a *OpenRouterAdapter) StreamComplete(ctx context.Context, req CompleteRequest, w io.Writer) (Usage, error) {
	if a.thinking {
		text, u, err := a.Complete(ctx, req)
		if err != nil {
			return Usage{}, err
		}
		return u, simulateStream(w, text)
	}

	body := openAIRequest{
		Model:     a.model,
		Messages:  a.buildMessages(req),
		Stream:    true,
		MaxTokens: req.MaxTokens,
	}

	resp, err := a.post(ctx, body)
	if err != nil {
		return Usage{}, fmt.Errorf("openrouter stream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return Usage{}, fmt.Errorf("openrouter %d: %s", resp.StatusCode, string(b))
	}

	return parseOpenAIStream(resp.Body, w)
}

// ── Chat / StreamChat ─────────────────────────────────────────────────────────

func (a *OpenRouterAdapter) Chat(ctx context.Context, req ChatRequest) (string, Usage, error) {
	body := openAIRequest{
		Model:     a.model,
		Messages:  a.buildChatMessages(req),
		Stream:    false,
		MaxTokens: req.MaxTokens,
	}

	resp, err := a.post(ctx, body)
	if err != nil {
		return "", Usage{}, fmt.Errorf("openrouter chat: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", Usage{}, fmt.Errorf("openrouter %d: %s", resp.StatusCode, string(b))
	}

	var result openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", Usage{}, fmt.Errorf("openrouter decode: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", Usage{}, fmt.Errorf("openrouter: no choices returned")
	}

	u := Usage{
		PromptTokens:     result.Usage.PromptTokens,
		CompletionTokens: result.Usage.CompletionTokens,
	}
	u.CostUSD = a.estimateCost(u.PromptTokens, u.CompletionTokens)
	return result.Choices[0].Message.Content, u, nil
}

func (a *OpenRouterAdapter) StreamChat(ctx context.Context, req ChatRequest, w io.Writer) (Usage, error) {
	body := openAIRequest{
		Model:     a.model,
		Messages:  a.buildChatMessages(req),
		Stream:    true,
		MaxTokens: req.MaxTokens,
	}

	resp, err := a.post(ctx, body)
	if err != nil {
		return Usage{}, fmt.Errorf("openrouter stream chat: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return Usage{}, fmt.Errorf("openrouter %d: %s", resp.StatusCode, string(b))
	}

	return parseOpenAIStream(resp.Body, w)
}

// ── Summarize ─────────────────────────────────────────────────────────────────

func (a *OpenRouterAdapter) Summarize(ctx context.Context, text, systemPrompt string) (string, Usage, error) {
	req := CompleteRequest{
		SystemPrompt: systemPrompt,
		Content:      text,
		Mode:         CompleteModeContinue,
		MaxTokens:    200,
	}
	return a.Complete(ctx, req)
}

// ── Tool use ──────────────────────────────────────────────────────────────────

// ChatTools implements ToolAdapter. OpenRouter supports the OpenAI tools API
// natively for all underlying providers that support function calling.
func (a *OpenRouterAdapter) ChatTools(ctx context.Context, msgs []Message, extraMsgs []json.RawMessage, tools []ToolDefinition, maxTokens int) (ToolChatResponse, error) {
	if maxTokens == 0 {
		maxTokens = 1024
	}

	rawMsgs := make([]json.RawMessage, 0, len(msgs)+len(extraMsgs))
	for _, m := range msgs {
		raw, _ := json.Marshal(map[string]string{"role": m.Role, "content": m.Content})
		rawMsgs = append(rawMsgs, raw)
	}
	rawMsgs = append(rawMsgs, extraMsgs...)

	oaiTools := make([]map[string]interface{}, len(tools))
	for i, t := range tools {
		oaiTools[i] = map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        t.Name,
				"description": t.Description,
				"parameters":  t.InputSchema,
			},
		}
	}

	reqMap := map[string]interface{}{
		"model":      a.model,
		"messages":   rawMsgs,
		"tools":      oaiTools,
		"max_tokens": maxTokens,
		"stream":     false,
	}

	data, err := json.Marshal(reqMap)
	if err != nil {
		return ToolChatResponse{}, err
	}

	resp, err := a.postJSON(ctx, data)
	if err != nil {
		return ToolChatResponse{}, fmt.Errorf("openrouter tools: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return ToolChatResponse{}, fmt.Errorf("openrouter %d: %s", resp.StatusCode, string(b))
	}

	var result openAIToolsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ToolChatResponse{}, fmt.Errorf("openrouter tools decode: %w", err)
	}
	if len(result.Choices) == 0 {
		return ToolChatResponse{}, fmt.Errorf("openrouter tools: no choices")
	}

	choice := result.Choices[0]
	out := ToolChatResponse{
		Usage: Usage{
			PromptTokens:     result.Usage.PromptTokens,
			CompletionTokens: result.Usage.CompletionTokens,
		},
	}
	out.Usage.CostUSD = a.estimateCost(out.Usage.PromptTokens, out.Usage.CompletionTokens)

	if choice.FinishReason == "tool_calls" {
		out.StopReason = "tool_use"
		for _, tc := range choice.Message.ToolCalls {
			out.ToolCalls = append(out.ToolCalls, ToolCall{
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: json.RawMessage(tc.Function.Arguments),
			})
		}
	} else {
		out.StopReason = "end_turn"
		if choice.Message.Content != nil {
			out.Text = *choice.Message.Content
		}
	}

	assistantMsg := map[string]interface{}{
		"role":       choice.Message.Role,
		"content":    choice.Message.Content,
		"tool_calls": choice.Message.ToolCalls,
	}
	if len(choice.Message.ToolCalls) == 0 {
		delete(assistantMsg, "tool_calls")
	}
	out.RawAssistantMsg, _ = json.Marshal(assistantMsg)

	return out, nil
}

// BuildToolResultMessages converts ToolResults into OpenAI-style tool-role messages.
func (a *OpenRouterAdapter) BuildToolResultMessages(results []ToolResult) []json.RawMessage {
	msgs := make([]json.RawMessage, len(results))
	for i, r := range results {
		msg := map[string]interface{}{
			"role":         "tool",
			"tool_call_id": r.ID,
			"content":      r.Content,
		}
		msgs[i], _ = json.Marshal(msg)
	}
	return msgs
}
