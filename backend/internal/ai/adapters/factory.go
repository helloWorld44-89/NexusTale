package adapters

import (
	"fmt"
	"strings"
)

// AdapterConfig holds provider-level configuration injected at startup.
type AdapterConfig struct {
	OllamaURL       string // default http://localhost:11434
	OllamaModel     string // default llama3.2
	OpenRouterModel string // default anthropic/claude-3-5-haiku
	GeminiModel     string // default gemini-2.0-flash
	GroqModel       string // default llama-3.1-70b-versatile
	DeepSeekModel   string // default deepseek-chat
}

// thinkingModelSubstrings are substrings that identify chain-of-thought models.
// These models: don't support system prompts in standard position, may not
// support streaming, and require max_completion_tokens instead of max_tokens.
var thinkingModelSubstrings = []string{
	"o1", "o3", "o4", "deepseek-reasoner", "qwq", "r1",
}

// isThinkingModel returns true when modelID contains a thinking-model substring.
func isThinkingModel(modelID string) bool {
	lower := strings.ToLower(modelID)
	for _, sub := range thinkingModelSubstrings {
		if strings.Contains(lower, sub) {
			return true
		}
	}
	return false
}

// NewAdapter constructs the adapter for the given provider and model.
// Falls back to Ollama when provider is "ollama" or apiKey is empty.
func NewAdapter(provider, apiKey, model string, cfg AdapterConfig) (Adapter, error) {
	if cfg.OllamaURL == "" {
		cfg.OllamaURL = "http://localhost:11434"
	}
	if cfg.OllamaModel == "" {
		cfg.OllamaModel = "llama3.2"
	}

	// Fall back to Ollama when no cloud key is available.
	if provider == "ollama" || apiKey == "" {
		m := cfg.OllamaModel
		if model != "" {
			m = model
		}
		return NewOllamaAdapter(cfg.OllamaURL, m), nil
	}

	switch provider {
	case "openai":
		m := "gpt-4o-mini"
		if model != "" {
			m = model
		}
		return NewOpenAIAdapter(apiKey, m), nil
	case "anthropic":
		m := "claude-haiku-4-5-20251001"
		if model != "" {
			m = model
		}
		return NewAnthropicAdapter(apiKey, m), nil
	case "openrouter":
		m := cfg.OpenRouterModel
		if model != "" {
			m = model
		}
		return NewOpenRouterAdapter(apiKey, m), nil
	case "gemini":
		m := cfg.GeminiModel
		if model != "" {
			m = model
		}
		return NewGeminiAdapter(apiKey, m), nil
	case "groq":
		m := cfg.GroqModel
		if model != "" {
			m = model
		}
		return NewGroqAdapter(apiKey, m), nil
	case "deepseek":
		m := cfg.DeepSeekModel
		if model != "" {
			m = model
		}
		return NewDeepSeekAdapter(apiKey, m), nil
	default:
		return nil, fmt.Errorf("unknown provider %q and no Ollama configured", provider)
	}
}
