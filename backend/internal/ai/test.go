package ai

// test.go — AI connection health-check. Provides TestConnection which pings
// the user's configured provider(s) without consuming generation tokens.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// TestResult is the response type for POST /ai/test-connection.
type TestResult struct {
	OK       bool     `json:"ok"`
	Provider string   `json:"provider"`
	Models   []string `json:"models,omitempty"`
	Error    string   `json:"error,omitempty"`
}

// pingTimeout caps how long a health-check request may take.
const pingTimeout = 8 * time.Second

// TestConnection verifies that the user's stored credentials for the given
// provider are reachable.
//
//   - ollama   → GET {stored_url}/api/tags; returns the model list
//   - openai   → GET https://api.openai.com/v1/models; returns model IDs
//   - anthropic → GET https://api.anthropic.com/v1/models; returns model IDs
//   - others   → confirms the key is stored; no live test
func (s *Service) TestConnection(ctx context.Context, userID uuid.UUID, provider string) TestResult {
	res := TestResult{Provider: provider}

	switch provider {
	case "ollama":
		url := s.ollamaURLForUser(ctx, userID)
		if url == "" {
			res.Error = "No Ollama URL configured. Save a URL in Settings first."
			return res
		}
		models, err := pingOllama(ctx, url)
		if err != nil {
			res.Error = err.Error()
			return res
		}
		res.OK = true
		res.Models = models

	case "openai":
		key, err := s.authSvc.DecryptAPIKey(ctx, userID, "openai")
		if err != nil {
			res.Error = "No OpenAI key stored."
			return res
		}
		models, err := pingOpenAI(ctx, key)
		if err != nil {
			res.Error = err.Error()
			return res
		}
		res.OK = true
		res.Models = models

	case "anthropic":
		key, err := s.authSvc.DecryptAPIKey(ctx, userID, "anthropic")
		if err != nil {
			res.Error = "No Anthropic key stored."
			return res
		}
		models, err := pingAnthropic(ctx, key)
		if err != nil {
			res.Error = err.Error()
			return res
		}
		res.OK = true
		res.Models = models

	default:
		// For gemini, mistral, custom — confirm key exists; no free test endpoint.
		if _, err := s.authSvc.DecryptAPIKey(ctx, userID, provider); err != nil {
			res.Error = fmt.Sprintf("No key stored for %q.", provider)
			return res
		}
		res.OK = true
		res.Models = []string{"key stored (live test not available for this provider)"}
	}

	return res
}

// ── provider pings ────────────────────────────────────────────────────────────

func pingOllama(ctx context.Context, baseURL string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()

	url := strings.TrimRight(baseURL, "/") + "/api/tags"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot reach Ollama at %s — check the URL and that Ollama is running", baseURL)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama returned HTTP %d", resp.StatusCode)
	}

	var body struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("unexpected Ollama response format: %w", err)
	}

	if len(body.Models) == 0 {
		return []string{"connected — no models pulled yet (run: ollama pull <model>)"}, nil
	}

	names := make([]string, 0, len(body.Models))
	for _, m := range body.Models {
		names = append(names, m.Name)
	}
	return names, nil
}

func pingOpenAI(ctx context.Context, key string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.openai.com/v1/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot reach OpenAI: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("OpenAI rejected the key — check it is valid and not expired")
	case http.StatusOK:
		// ok
	default:
		return nil, fmt.Errorf("OpenAI returned HTTP %d", resp.StatusCode)
	}

	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("unexpected OpenAI response: %w", err)
	}

	// Return a curated subset so the UI stays readable.
	keep := []string{}
	priority := []string{"gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "gpt-3.5-turbo"}
	seen := map[string]bool{}
	for _, want := range priority {
		for _, m := range body.Data {
			if m.ID == want && !seen[m.ID] {
				keep = append(keep, m.ID)
				seen[m.ID] = true
			}
		}
	}
	if len(keep) == 0 {
		keep = append(keep, fmt.Sprintf("%d models available", len(body.Data)))
	}
	return keep, nil
}

func pingAnthropic(ctx context.Context, key string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.anthropic.com/v1/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", key)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot reach Anthropic: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, fmt.Errorf("Anthropic rejected the key — check it is valid and not expired")
	case http.StatusOK:
		// ok
	default:
		return nil, fmt.Errorf("Anthropic returned HTTP %d", resp.StatusCode)
	}

	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("unexpected Anthropic response: %w", err)
	}

	names := make([]string, 0, len(body.Data))
	for _, m := range body.Data {
		names = append(names, m.ID)
	}
	return names, nil
}
