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

// Gemini is accessed via its OpenAI-compatible endpoint. No extra headers are
// required — standard Bearer auth works the same as OpenAI.
const geminiBaseURL = "https://generativelanguage.googleapis.com/v1beta/openai"

// Rough token prices for Gemini models (USD per token, paid tier).
// Free tier: gemini-2.0-flash = 15 RPM / 1M TPD; gemini-1.5-pro = 2 RPM / 50 RPD.
var geminiPricePerToken = map[string][2]float64{
	// [input $/token, output $/token]
	"gemini-2.0-flash":      {0.00000010, 0.00000040},
	"gemini-2.0-flash-lite": {0.000000075, 0.0000003},
	"gemini-1.5-flash":      {0.000000075, 0.0000003},
	"gemini-1.5-pro":        {0.00000125, 0.000005},
	"gemini-2.5-flash":      {0.00000015, 0.00000060},
	"gemini-2.5-pro":        {0.00000125, 0.000010},
}

// GeminiAdapter calls the Gemini API via its OpenAI-compatible endpoint.
// Default model is gemini-2.0-flash (fast, generous free tier).
// For the full 1M-token context window, use gemini-1.5-pro.
type GeminiAdapter struct {
	apiKey   string
	model    string
	thinking bool
	client   *http.Client
}

func NewGeminiAdapter(apiKey, model string) *GeminiAdapter {
	model = strings.TrimPrefix(model, "models/")
	if model == "" {
		model = "gemini-2.0-flash"
	}
	return &GeminiAdapter{
		apiKey:   apiKey,
		model:    model,
		thinking: isThinkingModel(model),
		client:   &http.Client{Timeout: 120 * time.Second},
	}
}

func (a *GeminiAdapter) Provider() string      { return "gemini" }
func (a *GeminiAdapter) IsThinkingModel() bool { return a.thinking }

// ── helpers ───────────────────────────────────────────────────────────────────

func (a *GeminiAdapter) buildMessages(req CompleteRequest) []openAIMessage {
	msgs := []openAIMessage{}
	if req.SystemPrompt != "" && !a.thinking {
		msgs = append(msgs, openAIMessage{Role: "system", Content: req.SystemPrompt})
	}
	content := req.Content
	if content == "" {
		// Gemini rejects requests with an empty user turn.
		content = "Continue the story."
	}
	msgs = append(msgs, openAIMessage{Role: "user", Content: content})
	return msgs
}

func (a *GeminiAdapter) buildChatMessages(req ChatRequest) []openAIMessage {
	msgs := make([]openAIMessage, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = openAIMessage{Role: m.Role, Content: m.Content}
	}
	return msgs
}

func (a *GeminiAdapter) estimateCost(prompt, completion int) float64 {
	prices, ok := geminiPricePerToken[a.model]
	if !ok {
		for k, v := range geminiPricePerToken {
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

func (a *GeminiAdapter) post(ctx context.Context, body openAIRequest) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return a.doRequest(ctx, data)
}

func (a *GeminiAdapter) postJSON(ctx context.Context, body []byte) (*http.Response, error) {
	return a.doRequest(ctx, body)
}

func (a *GeminiAdapter) doRequest(ctx context.Context, data []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		geminiBaseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")
	return a.client.Do(req)
}

// ── Complete ──────────────────────────────────────────────────────────────────

func (a *GeminiAdapter) Complete(ctx context.Context, req CompleteRequest) (string, Usage, error) {
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
		return "", Usage{}, fmt.Errorf("gemini request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", Usage{}, fmt.Errorf("gemini %d: %s", resp.StatusCode, string(b))
	}

	var result openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", Usage{}, fmt.Errorf("gemini decode: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", Usage{}, fmt.Errorf("gemini: no choices returned")
	}

	u := Usage{
		PromptTokens:     result.Usage.PromptTokens,
		CompletionTokens: result.Usage.CompletionTokens,
	}
	u.CostUSD = a.estimateCost(u.PromptTokens, u.CompletionTokens)
	return result.Choices[0].Message.Content, u, nil
}

// ── StreamComplete ────────────────────────────────────────────────────────────

func (a *GeminiAdapter) StreamComplete(ctx context.Context, req CompleteRequest, w io.Writer) (Usage, error) {
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
		return Usage{}, fmt.Errorf("gemini stream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return Usage{}, fmt.Errorf("gemini %d: %s", resp.StatusCode, string(b))
	}

	return parseOpenAIStream(resp.Body, w)
}

// ── Chat / StreamChat ─────────────────────────────────────────────────────────

func (a *GeminiAdapter) Chat(ctx context.Context, req ChatRequest) (string, Usage, error) {
	body := openAIRequest{
		Model:     a.model,
		Messages:  a.buildChatMessages(req),
		Stream:    false,
		MaxTokens: req.MaxTokens,
	}

	resp, err := a.post(ctx, body)
	if err != nil {
		return "", Usage{}, fmt.Errorf("gemini chat: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", Usage{}, fmt.Errorf("gemini %d: %s", resp.StatusCode, string(b))
	}

	var result openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", Usage{}, fmt.Errorf("gemini decode: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", Usage{}, fmt.Errorf("gemini: no choices returned")
	}

	u := Usage{
		PromptTokens:     result.Usage.PromptTokens,
		CompletionTokens: result.Usage.CompletionTokens,
	}
	u.CostUSD = a.estimateCost(u.PromptTokens, u.CompletionTokens)
	return result.Choices[0].Message.Content, u, nil
}

func (a *GeminiAdapter) StreamChat(ctx context.Context, req ChatRequest, w io.Writer) (Usage, error) {
	body := openAIRequest{
		Model:     a.model,
		Messages:  a.buildChatMessages(req),
		Stream:    true,
		MaxTokens: req.MaxTokens,
	}

	resp, err := a.post(ctx, body)
	if err != nil {
		return Usage{}, fmt.Errorf("gemini stream chat: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return Usage{}, fmt.Errorf("gemini %d: %s", resp.StatusCode, string(b))
	}

	return parseOpenAIStream(resp.Body, w)
}

// ── Summarize ─────────────────────────────────────────────────────────────────

func (a *GeminiAdapter) Summarize(ctx context.Context, text, systemPrompt string) (string, Usage, error) {
	req := CompleteRequest{
		SystemPrompt: systemPrompt,
		Content:      text,
		Mode:         CompleteModeContinue,
		MaxTokens:    200,
	}
	return a.Complete(ctx, req)
}

// ── Tool use ──────────────────────────────────────────────────────────────────

// ChatTools implements ToolAdapter using Gemini's OpenAI-compatible function
// calling API. Supported on gemini-1.5-pro, gemini-1.5-flash, gemini-2.0-flash,
// and gemini-2.5 series.
func (a *GeminiAdapter) ChatTools(ctx context.Context, msgs []Message, extraMsgs []json.RawMessage, tools []ToolDefinition, maxTokens int) (ToolChatResponse, error) {
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
		return ToolChatResponse{}, fmt.Errorf("gemini tools: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return ToolChatResponse{}, fmt.Errorf("gemini %d: %s", resp.StatusCode, string(b))
	}

	var result openAIToolsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ToolChatResponse{}, fmt.Errorf("gemini tools decode: %w", err)
	}
	if len(result.Choices) == 0 {
		return ToolChatResponse{}, fmt.Errorf("gemini tools: no choices")
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
func (a *GeminiAdapter) BuildToolResultMessages(results []ToolResult) []json.RawMessage {
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
