package adapters_test

// ollama_test.go — contract tests for OllamaAdapter.
//
// These are pure unit tests: they spin up an in-process httptest server that
// simulates Ollama responses. No real Ollama instance is required, and no DB
// connection is needed. They run on every `make test` / `go test ./...` call.
//
// What we verify:
//  1. The adapter sends the correct JSON shape (model, messages, stream flag).
//  2. The NDJSON stream parser correctly assembles SSE output.
//  3. Usage token counts are extracted from the final "done" chunk.
//  4. Non-200 responses return a clear error.
//  5. System prompt is mapped to a "system" role message.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jconder44/nexustale/internal/ai/adapters"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// ollamaStreamServer starts an httptest server that validates the request body
// then streams back the provided NDJSON chunks.
func ollamaStreamServer(t *testing.T, wantModel string, chunks []string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.Error(w, "wrong path", http.StatusNotFound)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
			http.Error(w, "wrong method", http.StatusMethodNotAllowed)
			return
		}

		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request body: %v", err)
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}

		if model, ok := body["model"].(string); !ok || model != wantModel {
			t.Errorf("expected model %q, got %q", wantModel, body["model"])
		}
		if stream, ok := body["stream"].(bool); !ok || !stream {
			t.Errorf("expected stream=true, got %v", body["stream"])
		}

		w.Header().Set("Content-Type", "application/x-ndjson")
		for _, chunk := range chunks {
			fmt.Fprintln(w, chunk)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
}

// ollamaChunk builds a minimal Ollama stream chunk JSON string.
func ollamaChunk(content string, done bool, promptTokens, evalTokens int) string {
	return fmt.Sprintf(`{"message":{"content":%q},"done":%v,"prompt_eval_count":%d,"eval_count":%d}`,
		content, done, promptTokens, evalTokens)
}

// ── StreamComplete ─────────────────────────────────────────────────────────────

func TestOllamaAdapter_StreamComplete_HappyPath(t *testing.T) {
	chunks := []string{
		ollamaChunk("The ", false, 0, 0),
		ollamaChunk("dragon ", false, 0, 0),
		ollamaChunk("flew.", false, 0, 0),
		ollamaChunk("", true, 42, 10),
	}
	srv := ollamaStreamServer(t, "mistral", chunks)
	defer srv.Close()

	adapter := adapters.NewOllamaAdapter(srv.URL, "mistral")
	var buf bytes.Buffer

	usage, err := adapter.StreamComplete(context.Background(), adapters.CompleteRequest{
		SystemPrompt: "You are a writer.",
		Content:      "Write a line about a dragon.",
	}, &buf)

	if err != nil {
		t.Fatalf("StreamComplete error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, `"delta":"The "`) {
		t.Errorf("expected SSE delta chunk, got: %s", out)
	}
	if !strings.Contains(out, `"delta":"flew."`) {
		t.Errorf("missing last word chunk, got: %s", out)
	}
	if !strings.Contains(out, "[DONE]") {
		t.Errorf("expected [DONE] sentinel, got: %s", out)
	}

	if usage.PromptTokens != 42 {
		t.Errorf("expected 42 prompt tokens, got %d", usage.PromptTokens)
	}
	if usage.CompletionTokens != 10 {
		t.Errorf("expected 10 completion tokens, got %d", usage.CompletionTokens)
	}
	// Ollama is local — cost should always be zero.
	if usage.CostUSD != 0 {
		t.Errorf("expected zero cost for Ollama, got %f", usage.CostUSD)
	}
}

func TestOllamaAdapter_StreamComplete_SystemPromptInMessages(t *testing.T) {
	// Verify the system prompt is sent as a "system" role message, not omitted.
	var capturedMessages []map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if msgs, ok := body["messages"].([]interface{}); ok {
			for _, m := range msgs {
				if msg, ok := m.(map[string]interface{}); ok {
					capturedMessages = append(capturedMessages, msg)
				}
			}
		}
		// Return a minimal done response.
		fmt.Fprintln(w, ollamaChunk("ok", true, 5, 3))
	}))
	defer srv.Close()

	adapter := adapters.NewOllamaAdapter(srv.URL, "llama3")
	var buf bytes.Buffer
	adapter.StreamComplete(context.Background(), adapters.CompleteRequest{
		SystemPrompt: "You are Nexus.",
		Content:      "Hello",
	}, &buf)

	if len(capturedMessages) < 2 {
		t.Fatalf("expected at least 2 messages (system + user), got %d", len(capturedMessages))
	}
	if capturedMessages[0]["role"] != "system" {
		t.Errorf("first message role should be 'system', got %q", capturedMessages[0]["role"])
	}
	if capturedMessages[0]["content"] != "You are Nexus." {
		t.Errorf("system message content mismatch: %q", capturedMessages[0]["content"])
	}
	if capturedMessages[1]["role"] != "user" {
		t.Errorf("second message role should be 'user', got %q", capturedMessages[1]["role"])
	}
}

func TestOllamaAdapter_StreamComplete_Non200ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"model not found"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	adapter := adapters.NewOllamaAdapter(srv.URL, "nonexistent-model")
	var buf bytes.Buffer
	_, err := adapter.StreamComplete(context.Background(), adapters.CompleteRequest{
		Content: "hello",
	}, &buf)

	if err == nil {
		t.Fatal("expected error for non-200 response, got nil")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected 404 in error message, got: %v", err)
	}
}

// ── StreamChat ────────────────────────────────────────────────────────────────

func TestOllamaAdapter_StreamChat_MultiTurnMessages(t *testing.T) {
	// Verify that all turns are forwarded to Ollama in order.
	var capturedMessages []map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if msgs, ok := body["messages"].([]interface{}); ok {
			for _, m := range msgs {
				if msg, ok := m.(map[string]interface{}); ok {
					capturedMessages = append(capturedMessages, msg)
				}
			}
		}
		fmt.Fprintln(w, ollamaChunk("response", true, 20, 8))
	}))
	defer srv.Close()

	adapter := adapters.NewOllamaAdapter(srv.URL, "mistral")
	var buf bytes.Buffer
	adapter.StreamChat(context.Background(), adapters.ChatRequest{
		Messages: []adapters.Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "What is the plot so far?"},
			{Role: "assistant", Content: "The hero has just arrived."},
			{Role: "user", Content: "What happens next?"},
		},
	}, &buf)

	if len(capturedMessages) != 4 {
		t.Errorf("expected 4 messages forwarded, got %d", len(capturedMessages))
	}
	// Spot-check order and content.
	for i, want := range []struct{ role, content string }{
		{"system", "You are a helpful assistant."},
		{"user", "What is the plot so far?"},
		{"assistant", "The hero has just arrived."},
		{"user", "What happens next?"},
	} {
		if capturedMessages[i]["role"] != want.role {
			t.Errorf("message[%d] role: expected %q, got %q", i, want.role, capturedMessages[i]["role"])
		}
		if capturedMessages[i]["content"] != want.content {
			t.Errorf("message[%d] content mismatch", i)
		}
	}
}

// ── MaxTokens ─────────────────────────────────────────────────────────────────

func TestOllamaAdapter_MaxTokens_SentAsNumPredict(t *testing.T) {
	var capturedOptions map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if opts, ok := body["options"].(map[string]interface{}); ok {
			capturedOptions = opts
		}
		fmt.Fprintln(w, ollamaChunk("ok", true, 0, 0))
	}))
	defer srv.Close()

	adapter := adapters.NewOllamaAdapter(srv.URL, "llama3")
	var buf bytes.Buffer
	adapter.StreamComplete(context.Background(), adapters.CompleteRequest{
		Content:   "Continue",
		MaxTokens: 512,
	}, &buf)

	if capturedOptions == nil {
		t.Fatal("expected options to be set when MaxTokens > 0")
	}
	numPredict, _ := capturedOptions["num_predict"].(float64) // JSON numbers decode as float64
	if int(numPredict) != 512 {
		t.Errorf("expected num_predict=512, got %v", capturedOptions["num_predict"])
	}
}

func TestOllamaAdapter_ZeroMaxTokens_NoOptionsField(t *testing.T) {
	var bodyOptions interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		bodyOptions = body["options"] // nil when absent
		fmt.Fprintln(w, ollamaChunk("ok", true, 0, 0))
	}))
	defer srv.Close()

	adapter := adapters.NewOllamaAdapter(srv.URL, "llama3")
	var buf bytes.Buffer
	adapter.StreamComplete(context.Background(), adapters.CompleteRequest{
		Content:   "Continue",
		MaxTokens: 0, // omit options
	}, &buf)

	if bodyOptions != nil {
		t.Errorf("expected options to be absent when MaxTokens=0, got: %v", bodyOptions)
	}
}

// ── Summarize ─────────────────────────────────────────────────────────────────

func TestOllamaAdapter_Summarize_UsesSystemPrompt(t *testing.T) {
	var capturedSystem string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if msgs, ok := body["messages"].([]interface{}); ok && len(msgs) > 0 {
			if msg, ok := msgs[0].(map[string]interface{}); ok {
				if msg["role"] == "system" {
					capturedSystem, _ = msg["content"].(string)
				}
			}
		}
		// Summarize uses Complete (non-streaming), returns a single JSON object.
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":           map[string]string{"content": "A short summary."},
			"done":              true,
			"prompt_eval_count": 30,
			"eval_count":        15,
		})
	}))
	defer srv.Close()

	adapter := adapters.NewOllamaAdapter(srv.URL, "llama3")
	summary, _, err := adapter.Summarize(context.Background(), "Long scene text here...", "Summarize the scene in 2-3 sentences.")

	if err != nil {
		t.Fatalf("Summarize error: %v", err)
	}
	if summary != "A short summary." {
		t.Errorf("expected summary, got %q", summary)
	}
	if !strings.Contains(capturedSystem, "Summarize") {
		t.Errorf("summarize system prompt should mention summarization, got: %q", capturedSystem)
	}
}

// ── Provider / IsThinkingModel ────────────────────────────────────────────────

func TestOllamaAdapter_ProviderName(t *testing.T) {
	adapter := adapters.NewOllamaAdapter("http://localhost:11434", "llama3")
	if adapter.Provider() != "ollama" {
		t.Errorf("expected provider 'ollama', got %q", adapter.Provider())
	}
}

func TestOllamaAdapter_IsNotThinkingModel(t *testing.T) {
	adapter := adapters.NewOllamaAdapter("http://localhost:11434", "llama3")
	if adapter.IsThinkingModel() {
		t.Error("Ollama adapter should never report IsThinkingModel=true")
	}
}

// ── Defaults ─────────────────────────────────────────────────────────────────

func TestOllamaAdapter_DefaultModel_WhenEmpty(t *testing.T) {
	var capturedModel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		capturedModel, _ = body["model"].(string)
		fmt.Fprintln(w, ollamaChunk("ok", true, 0, 0))
	}))
	defer srv.Close()

	// Passing empty model — should default to "llama3.2".
	adapter := adapters.NewOllamaAdapter(srv.URL, "")
	var buf bytes.Buffer
	adapter.StreamComplete(context.Background(), adapters.CompleteRequest{Content: "hi"}, &buf)

	if capturedModel != "llama3.2" {
		t.Errorf("expected default model 'llama3.2', got %q", capturedModel)
	}
}

// ── Interface compliance ──────────────────────────────────────────────────────

// Compile-time check: OllamaAdapter must satisfy the Adapter interface.
var _ adapters.Adapter = (*adapters.OllamaAdapter)(nil)

// Ensure the Adapter methods are callable via the interface.
func TestOllamaAdapter_ImplementsAdapterInterface(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, ollamaChunk("hi", true, 1, 1))
	}))
	defer srv.Close()

	var a adapters.Adapter = adapters.NewOllamaAdapter(srv.URL, "llama3")
	if a.Provider() == "" {
		t.Error("Provider() should not be empty")
	}
	var buf bytes.Buffer
	a.StreamComplete(context.Background(), adapters.CompleteRequest{Content: "test"}, io.Discard)
	_ = buf
}
