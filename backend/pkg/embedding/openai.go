package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// OpenAIEmbedder calls the OpenAI /v1/embeddings endpoint.
// Uses text-embedding-3-small with dimensions=768 so output matches Dims.
type OpenAIEmbedder struct {
	apiKey string
	client *http.Client
}

func NewOpenAIEmbedder(apiKey string) *OpenAIEmbedder {
	return &OpenAIEmbedder{apiKey: apiKey, client: &http.Client{}}
}

func (e *OpenAIEmbedder) Provider() string { return "openai" }

func (e *OpenAIEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	payload, _ := json.Marshal(map[string]any{
		"model":      "text-embedding-3-small",
		"input":      text,
		"dimensions": Dims,
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.openai.com/v1/embeddings", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai embed request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai embed: status %d", resp.StatusCode)
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("openai embed decode: %w", err)
	}
	if len(result.Data) == 0 || len(result.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("openai embed: empty response")
	}
	return result.Data[0].Embedding, nil
}
