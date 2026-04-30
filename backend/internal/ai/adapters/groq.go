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

const groqBaseURL = "https://api.groq.com/openai/v1"

// Rough token prices for Groq models (USD per token, paid tier).
// Free tier is generous; prices apply once the daily limit is exceeded.
var groqPricePerToken = map[string][2]float64{
	"llama-3.1-70b-versatile": {0.00000059, 0.00000079},
	"llama-3.1-8b-instant":    {0.00000005, 0.00000008},
	"llama3-70b-8192":         {0.00000059, 0.00000079},
	"llama3-8b-8192":          {0.00000005, 0.00000008},
	"mixtral-8x7b-32768":      {0.00000024, 0.00000024},
	"gemma2-9b-it":            {0.00000020, 0.00000020},
}

// GroqAdapter calls the Groq Cloud API (OpenAI wire format).
// Default model is llama-3.1-70b-versatile — the fastest large-context option
// on the free tier (~500 tokens/second; Beat mode feels near-instant).
type GroqAdapter struct {
	apiKey   string
	model    string
	thinking bool
	client   *http.Client
}

func NewGroqAdapter(apiKey, model string) *GroqAdapter {
	if model == "" {
		model = "llama-3.1-70b-versatile"
	}
	return &GroqAdapter{
		apiKey:   apiKey,
		model:    model,
		thinking: isThinkingModel(model),
		client:   &http.Client{Timeout: 120 * time.Second},
	}
}

func (a *GroqAdapter) Provider() string      { return "groq" }
func (a *GroqAdapter) IsThinkingModel() bool { return a.thinking }

// ── helpers ───────────────────────────────────────────────────────────────────

func (a *GroqAdapter) buildMessages(req CompleteRequest) []openAIMessage {
	msgs := []openAIMessage{}
	if req.SystemPrompt != "" && !a.thinking {
		msgs = append(msgs, openAIMessage{Role: "system", Content: req.SystemPrompt})
	}
	msgs = append(msgs, openAIMessage{Role: "user", Content: req.Content})
	return msgs
}

func (a *GroqAdapter) buildChatMessages(req ChatRequest) []openAIMessage {
	msgs := make([]openAIMessage, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = openAIMessage{Role: m.Role, Content: m.Content}
	}
	return msgs
}

func (a *GroqAdapter) estimateCost(prompt, completion int) float64 {
	prices, ok := groqPricePerToken[a.model]
	if !ok {
		for k, v := range groqPricePerToken {
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

func (a *GroqAdapter) post(ctx context.Context, body openAIRequest) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return a.doRequest(ctx, data)
}

func (a *GroqAdapter) postJSON(ctx context.Context, body []byte) (*http.Response, error) {
	return a.doRequest(ctx, body)
}

func (a *GroqAdapter) doRequest(ctx context.Context, data []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		groqBaseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")
	return a.client.Do(req)
}

// ── Complete ──────────────────────────────────────────────────────────────────

func (a *GroqAdapter) Complete(ctx context.Context, req CompleteRequest) (string, Usage, error) {
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
		return "", Usage{}, fmt.Errorf("groq request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", Usage{}, fmt.Errorf("groq %d: %s", resp.StatusCode, string(b))
	}

	var result openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", Usage{}, fmt.Errorf("groq decode: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", Usage{}, fmt.Errorf("groq: no choices returned")
	}

	u := Usage{
		PromptTokens:     result.Usage.PromptTokens,
		CompletionTokens: result.Usage.CompletionTokens,
	}
	u.CostUSD = a.estimateCost(u.PromptTokens, u.CompletionTokens)
	return result.Choices[0].Message.Content, u, nil
}

// ── StreamComplete ────────────────────────────────────────────────────────────

func (a *GroqAdapter) StreamComplete(ctx context.Context, req CompleteRequest, w io.Writer) (Usage, error) {
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
		return Usage{}, fmt.Errorf("groq stream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return Usage{}, fmt.Errorf("groq %d: %s", resp.StatusCode, string(b))
	}

	return parseOpenAIStream(resp.Body, w)
}

// ── Chat / StreamChat ─────────────────────────────────────────────────────────

func (a *GroqAdapter) Chat(ctx context.Context, req ChatRequest) (string, Usage, error) {
	body := openAIRequest{
		Model:     a.model,
		Messages:  a.buildChatMessages(req),
		Stream:    false,
		MaxTokens: req.MaxTokens,
	}

	resp, err := a.post(ctx, body)
	if err != nil {
		return "", Usage{}, fmt.Errorf("groq chat: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", Usage{}, fmt.Errorf("groq %d: %s", resp.StatusCode, string(b))
	}

	var result openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", Usage{}, fmt.Errorf("groq decode: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", Usage{}, fmt.Errorf("groq: no choices returned")
	}

	u := Usage{
		PromptTokens:     result.Usage.PromptTokens,
		CompletionTokens: result.Usage.CompletionTokens,
	}
	u.CostUSD = a.estimateCost(u.PromptTokens, u.CompletionTokens)
	return result.Choices[0].Message.Content, u, nil
}

func (a *GroqAdapter) StreamChat(ctx context.Context, req ChatRequest, w io.Writer) (Usage, error) {
	body := openAIRequest{
		Model:     a.model,
		Messages:  a.buildChatMessages(req),
		Stream:    true,
		MaxTokens: req.MaxTokens,
	}

	resp, err := a.post(ctx, body)
	if err != nil {
		return Usage{}, fmt.Errorf("groq stream chat: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return Usage{}, fmt.Errorf("groq %d: %s", resp.StatusCode, string(b))
	}

	return parseOpenAIStream(resp.Body, w)
}

// ── Summarize ─────────────────────────────────────────────────────────────────

func (a *GroqAdapter) Summarize(ctx context.Context, text, systemPrompt string) (string, Usage, error) {
	req := CompleteRequest{
		SystemPrompt: systemPrompt,
		Content:      text,
		Mode:         CompleteModeContinue,
		MaxTokens:    200,
	}
	return a.Complete(ctx, req)
}

// ── Tool use ──────────────────────────────────────────────────────────────────

func (a *GroqAdapter) ChatTools(ctx context.Context, msgs []Message, extraMsgs []json.RawMessage, tools []ToolDefinition, maxTokens int) (ToolChatResponse, error) {
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
		return ToolChatResponse{}, fmt.Errorf("groq tools: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return ToolChatResponse{}, fmt.Errorf("groq %d: %s", resp.StatusCode, string(b))
	}

	var result openAIToolsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ToolChatResponse{}, fmt.Errorf("groq tools decode: %w", err)
	}
	if len(result.Choices) == 0 {
		return ToolChatResponse{}, fmt.Errorf("groq tools: no choices")
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

func (a *GroqAdapter) BuildToolResultMessages(results []ToolResult) []json.RawMessage {
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
