package adapters

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

const openAIImageModel = "gpt-image-1"

// imageCostUSD is a flat per-image estimate for gpt-image-1 at default
// (medium quality, 1024x1024) settings. Not authoritative — display only,
// same caveat as openAIPricePerToken in openai.go.
const imageCostUSD = 0.04

// OpenAIImageAdapter calls OpenAI's Images API (gpt-image-1).
type OpenAIImageAdapter struct {
	apiKey string
	client *http.Client
}

func NewOpenAIImageAdapter(apiKey string) *OpenAIImageAdapter {
	return &OpenAIImageAdapter{
		apiKey: apiKey,
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

func (a *OpenAIImageAdapter) Provider() string { return "openai" }

type openAIImageResponse struct {
	Data []struct {
		B64JSON string `json:"b64_json"`
	} `json:"data"`
}

// GenerateImage calls /v1/images/generations for a first draft (no
// reference), or /v1/images/edits to revise req.ReferenceImage.
func (a *OpenAIImageAdapter) GenerateImage(ctx context.Context, req ImageRequest) (ImageResult, Usage, error) {
	var resp *http.Response
	var err error
	if req.ReferenceImage == nil {
		resp, err = a.postGenerate(ctx, req.Prompt)
	} else {
		resp, err = a.postEdit(ctx, req.Prompt, req.ReferenceImage)
	}
	if err != nil {
		return ImageResult{}, Usage{}, fmt.Errorf("openai image request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ImageResult{}, Usage{}, fmt.Errorf("openai image read: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return ImageResult{}, Usage{}, openAIErrorMessage("OpenAI", resp.StatusCode, body)
	}

	var result openAIImageResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return ImageResult{}, Usage{}, fmt.Errorf("openai image decode: %w", err)
	}
	if len(result.Data) == 0 || result.Data[0].B64JSON == "" {
		return ImageResult{}, Usage{}, fmt.Errorf("openai image: no image returned")
	}

	imgBytes, err := base64.StdEncoding.DecodeString(result.Data[0].B64JSON)
	if err != nil {
		return ImageResult{}, Usage{}, fmt.Errorf("openai image base64 decode: %w", err)
	}

	return ImageResult{ImageBytes: imgBytes, ContentType: "image/png"},
		Usage{CostUSD: imageCostUSD}, nil
}

func (a *OpenAIImageAdapter) postGenerate(ctx context.Context, prompt string) (*http.Response, error) {
	body, err := json.Marshal(map[string]string{
		"model":  openAIImageModel,
		"prompt": prompt,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		openAIBaseURL+"/images/generations", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")
	return a.client.Do(req)
}

func (a *OpenAIImageAdapter) postEdit(ctx context.Context, prompt string, referenceImage []byte) (*http.Response, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	if err := mw.WriteField("model", openAIImageModel); err != nil {
		return nil, err
	}
	if err := mw.WriteField("prompt", prompt); err != nil {
		return nil, err
	}
	fw, err := mw.CreateFormFile("image", "reference.png")
	if err != nil {
		return nil, err
	}
	if _, err := fw.Write(referenceImage); err != nil {
		return nil, err
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		openAIBaseURL+"/images/edits", &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return a.client.Do(req)
}
