package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// OllamaEmbedder calls the Ollama /api/embeddings endpoint.
// Defaults to nomic-embed-text which produces 768-dim vectors matching Dims.
const defaultOllamaEmbedModel = "nomic-embed-text"

type OllamaEmbedder struct {
	baseURL string
	model   string
	client  *http.Client
}

func NewOllamaEmbedder(baseURL, model string) *OllamaEmbedder {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if model == "" {
		model = defaultOllamaEmbedModel
	}
	return &OllamaEmbedder{baseURL: baseURL, model: model, client: &http.Client{}}
}

func (e *OllamaEmbedder) Provider() string { return "ollama" }

func (e *OllamaEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	payload, _ := json.Marshal(map[string]string{
		"model":  e.model,
		"prompt": text,
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		e.baseURL+"/api/embeddings", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama embed request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama embed: status %d", resp.StatusCode)
	}

	var result struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("ollama embed decode: %w", err)
	}
	if len(result.Embedding) == 0 {
		return nil, fmt.Errorf("ollama embed: empty embedding (is %s pulled?)", e.model)
	}
	return result.Embedding, nil
}
