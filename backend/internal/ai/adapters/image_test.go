package adapters_test

// image_test.go — contract tests for the image-adapter factory.
//
// OpenAIImageAdapter targets a hardcoded base URL (same as the existing text
// OpenAIAdapter, which has no HTTP-level test either — only OllamaAdapter's
// configurable URL gets an httptest-backed contract test). These tests cover
// the parts that don't require network access: provider selection and error
// handling in NewImageAdapter.

import (
	"testing"

	"github.com/jconder44/nexustale/internal/ai/adapters"
)

func TestNewImageAdapterUnknownProvider(t *testing.T) {
	if _, err := adapters.NewImageAdapter("stability", "key", ""); err == nil {
		t.Fatal("expected error for unknown image provider, got nil")
	}
}

func TestNewImageAdapterMissingKey(t *testing.T) {
	if _, err := adapters.NewImageAdapter("openai", "", ""); err == nil {
		t.Fatal("expected error for missing API key, got nil")
	}
}

func TestNewImageAdapterOpenAI(t *testing.T) {
	adapter, err := adapters.NewImageAdapter("openai", "sk-test", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if adapter.Provider() != "openai" {
		t.Errorf("Provider() = %q, want %q", adapter.Provider(), "openai")
	}
}

