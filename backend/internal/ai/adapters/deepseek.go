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

const deepSeekBaseURL = "https://api.deepseek.com/v1"

// Rough token prices for DeepSeek models (USD per token, cache-miss rate).
// deepseek-chat (V3) is GPT-4o-class quality at ~3% of the cost.
// deepseek-reasoner (R1) is a thinking model — uses chain-of-thought and does
// not support streaming or standard system prompts.
var deepSeekPricePerToken = map[string][2]float64{
	"deepseek-chat":     {0.00000027, 0.0000011},
	"deepseek-reasoner": {0.00000055, 0.00000219},
}

// DeepSeekAdapter calls the DeepSeek API (OpenAI wire format).
// Default model is deepseek-chat (V3) — best price-to-quality ratio available.
//
// Privacy note: DeepSeek servers are operated by a Chinese company. Writers
// with data-sensitivity concerns should use Anthropic, OpenAI, Gemini, or Ollama.
type DeepSeekAdapter struct {
	apiKey   string
	model    string
	thinking bool
	client   *http.Client
}

func NewDeepSeekAdapter(apiKey, model string) *DeepSeekAdapter {
	if model == "" {
		model = "deepseek-chat"
	}
	return &DeepSeekAdapter{
		apiKey:   apiKey,
		model:    model,
		thinking: isThinkingModel(model),
		client:   &http.Client{Timeout: 120 * time.Second},
	}
}

func (a *DeepSeekAdapter) Provider() string      { return "deepseek" }
func (a *DeepSeekAdapter) IsThinkingModel() bool { return a.thinking }

// ── helpers ───────────────────────────────────────────────────────────────────

func (a *DeepSeekAdapter) buildMessages(req CompleteRequest) []openAIMessage {
	msgs := []openAIMessage{}
	if req.SystemPrompt != "" && !a.thinking {
		msgs = append(msgs, openAIMessage{Role: "system", Content: req.SystemPrompt})
	}
	msgs = append(msgs, openAIMessage{Role: "user", Content: req.Content})
	return msgs
}

func (a *DeepSeekAdapter) buildChatMessages(req ChatRequest) []openAIMessage {
	msgs := make([]openAIMessage, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = openAIMessage{Role: m.Role, Content: m.Content}
	}
	return msgs
}

func (a *DeepSeekAdapter) estimateCost(prompt, completion int) float64 {
	prices, ok := deepSeekPricePerToken[a.model]
	if !ok {
		for k, v := range deepSeekPricePerToken {
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

func (a *DeepSeekAdapter) post(ctx context.Context, body openAIRequest) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return a.doRequest(ctx, data)
}

func (a *DeepSeekAdapter) postJSON(ctx context.Context, body []byte) (*http.Response, error) {
	return a.doRequest(ctx, body)
}

func (a *DeepSeekAdapter) doRequest(ctx context.Context, data []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		deepSeekBaseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")
	return a.client.Do(req)
}

// ── Complete ──────────────────────────────────────────────────────────────────

func (a *DeepSeekAdapter) Complete(ctx context.Context, req CompleteRequest) (string, Usage, error) {
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
		return "", Usage{}, fmt.Errorf("deepseek request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", Usage{}, fmt.Errorf("deepseek %d: %s", resp.StatusCode, string(b))
	}

	var result openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", Usage{}, fmt.Errorf("deepseek decode: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", Usage{}, fmt.Errorf("deepseek: no choices returned")
	}

	u := Usage{
		PromptTokens:     result.Usage.PromptTokens,
		CompletionTokens: result.Usage.CompletionTokens,
	}
	u.CostUSD = a.estimateCost(u.PromptTokens, u.CompletionTokens)
	return result.Choices[0].Message.Content, u, nil
}

// ── StreamComplete ────────────────────────────────────────────────────────────

func (a *DeepSeekAdapter) StreamComplete(ctx context.Context, req CompleteRequest, w io.Writer) (Usage, error) {
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
		return Usage{}, fmt.Errorf("deepseek stream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return Usage{}, fmt.Errorf("deepseek %d: %s", resp.StatusCode, string(b))
	}

	return parseOpenAIStream(resp.Body, w)
}

// ── Chat / StreamChat ─────────────────────────────────────────────────────────

func (a *DeepSeekAdapter) Chat(ctx context.Context, req ChatRequest) (string, Usage, error) {
	body := openAIRequest{
		Model:     a.model,
		Messages:  a.buildChatMessages(req),
		Stream:    false,
		MaxTokens: req.MaxTokens,
	}

	resp, err := a.post(ctx, body)
	if err != nil {
		return "", Usage{}, fmt.Errorf("deepseek chat: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", Usage{}, fmt.Errorf("deepseek %d: %s", resp.StatusCode, string(b))
	}

	var result openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", Usage{}, fmt.Errorf("deepseek decode: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", Usage{}, fmt.Errorf("deepseek: no choices returned")
	}

	u := Usage{
		PromptTokens:     result.Usage.PromptTokens,
		CompletionTokens: result.Usage.CompletionTokens,
	}
	u.CostUSD = a.estimateCost(u.PromptTokens, u.CompletionTokens)
	return result.Choices[0].Message.Content, u, nil
}

func (a *DeepSeekAdapter) StreamChat(ctx context.Context, req ChatRequest, w io.Writer) (Usage, error) {
	body := openAIRequest{
		Model:     a.model,
		Messages:  a.buildChatMessages(req),
		Stream:    true,
		MaxTokens: req.MaxTokens,
	}

	resp, err := a.post(ctx, body)
	if err != nil {
		return Usage{}, fmt.Errorf("deepseek stream chat: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return Usage{}, fmt.Errorf("deepseek %d: %s", resp.StatusCode, string(b))
	}

	return parseOpenAIStream(resp.Body, w)
}

// ── Summarize ─────────────────────────────────────────────────────────────────

func (a *DeepSeekAdapter) Summarize(ctx context.Context, text string) (string, Usage, error) {
	req := CompleteRequest{
		SystemPrompt: "You are a writing assistant. Summarize the following scene or chapter content in 2–3 sentences, focusing on key plot events, character decisions, and narrative momentum. Be concise and factual.",
		Content:      text,
		Mode:         CompleteModeContinue,
		MaxTokens:    200,
	}
	return a.Complete(ctx, req)
}

// ── Tool use ──────────────────────────────────────────────────────────────────

// ChatTools implements ToolAdapter. deepseek-chat supports function calling;
// deepseek-reasoner does not — callers should check IsThinkingModel() and
// fall back to StreamChat for reasoner sessions.
func (a *DeepSeekAdapter) ChatTools(ctx context.Context, msgs []Message, extraMsgs []json.RawMessage, tools []ToolDefinition, maxTokens int) (ToolChatResponse, error) {
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
		return ToolChatResponse{}, fmt.Errorf("deepseek tools: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return ToolChatResponse{}, fmt.Errorf("deepseek %d: %s", resp.StatusCode, string(b))
	}

	var result openAIToolsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ToolChatResponse{}, fmt.Errorf("deepseek tools decode: %w", err)
	}
	if len(result.Choices) == 0 {
		return ToolChatResponse{}, fmt.Errorf("deepseek tools: no choices")
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

func (a *DeepSeekAdapter) BuildToolResultMessages(results []ToolResult) []json.RawMessage {
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
