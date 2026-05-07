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

const openAIBaseURL = "https://api.openai.com/v1"

// Rough token prices in USD per token (not authoritative — for display only).
var openAIPricePerToken = map[string][2]float64{
	// [input $/token, output $/token]
	"gpt-4o":        {0.0000025, 0.000010},
	"gpt-4o-mini":   {0.00000015, 0.0000006},
	"gpt-4-turbo":   {0.000010, 0.000030},
	"gpt-3.5-turbo": {0.0000005, 0.0000015},
}

// OpenAIAdapter calls the OpenAI Chat Completions API.
type OpenAIAdapter struct {
	apiKey       string
	model        string
	thinking     bool
	client       *http.Client
}

func NewOpenAIAdapter(apiKey, model string) *OpenAIAdapter {
	return &OpenAIAdapter{
		apiKey:   apiKey,
		model:    model,
		thinking: isThinkingModel(model),
		client:   &http.Client{Timeout: 120 * time.Second},
	}
}

func (a *OpenAIAdapter) Provider() string      { return "openai" }
func (a *OpenAIAdapter) IsThinkingModel() bool { return a.thinking }

// ── request/response shapes ───────────────────────────────────────────────────

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIRequest struct {
	Model               string          `json:"model"`
	Messages            []openAIMessage `json:"messages"`
	MaxTokens           int             `json:"max_tokens,omitempty"`
	MaxCompletionTokens int             `json:"max_completion_tokens,omitempty"` // o1/o3
	Stream              bool            `json:"stream"`
	Temperature         *float64        `json:"temperature,omitempty"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

type openAIStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

// ── helpers ───────────────────────────────────────────────────────────────────

func (a *OpenAIAdapter) buildMessages(req CompleteRequest) []openAIMessage {
	msgs := []openAIMessage{}
	// Thinking models don't support the system role in standard position.
	if req.SystemPrompt != "" && !a.thinking {
		msgs = append(msgs, openAIMessage{Role: "system", Content: req.SystemPrompt})
	}
	msgs = append(msgs, openAIMessage{Role: "user", Content: req.Content})
	return msgs
}

func (a *OpenAIAdapter) buildChatMessages(req ChatRequest) []openAIMessage {
	msgs := make([]openAIMessage, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = openAIMessage{Role: m.Role, Content: m.Content}
	}
	return msgs
}

func (a *OpenAIAdapter) estimateCost(prompt, completion int) float64 {
	// Check exact match first, then prefix match.
	prices, ok := openAIPricePerToken[a.model]
	if !ok {
		for k, v := range openAIPricePerToken {
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

func (a *OpenAIAdapter) post(ctx context.Context, body openAIRequest) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		openAIBaseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")
	return a.client.Do(req)
}

// ── Complete (non-streaming) ──────────────────────────────────────────────────

func (a *OpenAIAdapter) Complete(ctx context.Context, req CompleteRequest) (string, Usage, error) {
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
		return "", Usage{}, fmt.Errorf("openai request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", Usage{}, openAIErrorMessage("OpenAI", resp.StatusCode, b)
	}

	var result openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", Usage{}, fmt.Errorf("openai decode: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", Usage{}, fmt.Errorf("openai: no choices returned")
	}

	u := Usage{
		PromptTokens:     result.Usage.PromptTokens,
		CompletionTokens: result.Usage.CompletionTokens,
	}
	u.CostUSD = a.estimateCost(u.PromptTokens, u.CompletionTokens)
	return result.Choices[0].Message.Content, u, nil
}

// ── StreamComplete ────────────────────────────────────────────────────────────

// StreamComplete writes NexusTale SSE lines to w:
//
//	data: {"delta":"word "}\n\n
//	data: [DONE]\n\n
//
// For thinking models it calls Complete and simulates streaming by writing the
// full response word-by-word with brief pauses (no actual sleep — just a single
// bulk write, the real delay is the LLM response time).
func (a *OpenAIAdapter) StreamComplete(ctx context.Context, req CompleteRequest, w io.Writer) (Usage, error) {
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
		return Usage{}, fmt.Errorf("openai stream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return Usage{}, openAIErrorMessage("OpenAI", resp.StatusCode, b)
	}

	return parseOpenAIStream(resp.Body, w)
}

// ── Chat / StreamChat ─────────────────────────────────────────────────────────

func (a *OpenAIAdapter) Chat(ctx context.Context, req ChatRequest) (string, Usage, error) {
	body := openAIRequest{
		Model:     a.model,
		Messages:  a.buildChatMessages(req),
		Stream:    false,
		MaxTokens: req.MaxTokens,
	}

	resp, err := a.post(ctx, body)
	if err != nil {
		return "", Usage{}, fmt.Errorf("openai chat: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", Usage{}, openAIErrorMessage("OpenAI", resp.StatusCode, b)
	}

	var result openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", Usage{}, fmt.Errorf("openai decode: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", Usage{}, fmt.Errorf("openai: no choices returned")
	}

	u := Usage{
		PromptTokens:     result.Usage.PromptTokens,
		CompletionTokens: result.Usage.CompletionTokens,
	}
	u.CostUSD = a.estimateCost(u.PromptTokens, u.CompletionTokens)
	return result.Choices[0].Message.Content, u, nil
}

func (a *OpenAIAdapter) StreamChat(ctx context.Context, req ChatRequest, w io.Writer) (Usage, error) {
	body := openAIRequest{
		Model:     a.model,
		Messages:  a.buildChatMessages(req),
		Stream:    true,
		MaxTokens: req.MaxTokens,
	}

	resp, err := a.post(ctx, body)
	if err != nil {
		return Usage{}, fmt.Errorf("openai stream chat: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return Usage{}, openAIErrorMessage("OpenAI", resp.StatusCode, b)
	}

	return parseOpenAIStream(resp.Body, w)
}

// ── Summarize ─────────────────────────────────────────────────────────────────

func (a *OpenAIAdapter) Summarize(ctx context.Context, text, systemPrompt string) (string, Usage, error) {
	req := CompleteRequest{
		SystemPrompt: systemPrompt,
		Content:      text,
		Mode:         CompleteModeContinue,
		MaxTokens:    200,
	}
	return a.Complete(ctx, req)
}

// ── tool use ──────────────────────────────────────────────────────────────────

// openAIToolCall is one tool invocation in an OpenAI tool-calling response.
type openAIToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// openAIToolsResponse is the non-streaming API response when tools are present.
type openAIToolsResponse struct {
	Choices []struct {
		Message struct {
			Role      string           `json:"role"`
			Content   *string          `json:"content"`
			ToolCalls []openAIToolCall  `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

// postJSON sends an arbitrary JSON body to the OpenAI chat completions endpoint.
func (a *OpenAIAdapter) postJSON(ctx context.Context, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		openAIBaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")
	return a.client.Do(req)
}

// ChatTools implements ToolAdapter. It runs one non-streaming function-calling
// round using the OpenAI tools API and returns the model's response.
func (a *OpenAIAdapter) ChatTools(ctx context.Context, msgs []Message, extraMsgs []json.RawMessage, tools []ToolDefinition, maxTokens int) (ToolChatResponse, error) {
	if maxTokens == 0 {
		maxTokens = 1024
	}

	// Build the message array: normal messages first, then extra (tool history).
	rawMsgs := make([]json.RawMessage, 0, len(msgs)+len(extraMsgs))
	for _, m := range msgs {
		raw, _ := json.Marshal(map[string]string{"role": m.Role, "content": m.Content})
		rawMsgs = append(rawMsgs, raw)
	}
	rawMsgs = append(rawMsgs, extraMsgs...)

	// Convert ToolDefinitions to OpenAI's {type:"function", function:{...}} shape.
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
		return ToolChatResponse{}, fmt.Errorf("openai tools: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return ToolChatResponse{}, openAIErrorMessage("OpenAI", resp.StatusCode, b)
	}

	var result openAIToolsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ToolChatResponse{}, fmt.Errorf("openai tools decode: %w", err)
	}
	if len(result.Choices) == 0 {
		return ToolChatResponse{}, fmt.Errorf("openai tools: no choices")
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

	// Preserve the assistant message for replay — OpenAI requires the full
	// tool_calls array in the history or the next round will reject it.
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

// BuildToolResultMessages converts ToolResults into OpenAI tool-role messages —
// one per result (OpenAI requires individual tool messages, not a batch).
func (a *OpenAIAdapter) BuildToolResultMessages(results []ToolResult) []json.RawMessage {
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

// ── SSE parser ────────────────────────────────────────────────────────────────

// parseOpenAIStream reads the OpenAI SSE response and writes NexusTale SSE format.
// The scanner buffer is set to 1 MB to handle long thinking-model lines
// (Gemini 2.5 Flash can send entire reasoning traces as a single SSE line).
func parseOpenAIStream(body io.Reader, w io.Writer) (Usage, error) {
	var u Usage
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 1<<20), 1<<20) // 1 MB max line
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			fmt.Fprintf(w, "data: [DONE]\n\n")
			return u, nil
		}

		var chunk openAIStreamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}

		// Capture final usage if present (stream_options: include_usage).
		if chunk.Usage != nil {
			u.PromptTokens = chunk.Usage.PromptTokens
			u.CompletionTokens = chunk.Usage.CompletionTokens
		}

		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta.Content
		if delta == "" {
			continue
		}

		encoded, _ := json.Marshal(map[string]string{"delta": delta})
		fmt.Fprintf(w, "data: %s\n\n", encoded)
	}
	// Always close the stream so the client doesn't hang.
	fmt.Fprintf(w, "data: [DONE]\n\n")
	return u, scanner.Err()
}

// simulateStream writes a full text response as a single SSE event.
// Used for thinking models that don't support true streaming.
func simulateStream(w io.Writer, text string) error {
	encoded, _ := json.Marshal(map[string]string{"delta": text})
	fmt.Fprintf(w, "data: %s\n\n", encoded)
	fmt.Fprintf(w, "data: [DONE]\n\n")
	return nil
}

// openAIErrorMessage parses the OpenAI-compatible error envelope used by
// OpenAI, Groq, OpenRouter, and DeepSeek:
//
//	{"error": {"message": "...", "type": "...", "code": "..."}}
//
// Falls back to the raw body if parsing fails.
func openAIErrorMessage(provider string, status int, body []byte) error {
	var envelope struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    any    `json:"code"` // string or int depending on provider
		} `json:"error"`
	}
	if json.Unmarshal(body, &envelope) == nil && envelope.Error.Message != "" {
		msg := envelope.Error.Message
		if status == 429 {
			return fmt.Errorf("%s quota/rate-limit exceeded: %s", provider, msg)
		}
		if status == 401 || status == 403 {
			return fmt.Errorf("%s authentication error: %s", provider, msg)
		}
		return fmt.Errorf("%s error %d: %s", provider, status, msg)
	}
	return fmt.Errorf("%s %d: %s", provider, status, string(body))
}
