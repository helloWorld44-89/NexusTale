package adapters

import "fmt"

// NewImageAdapter constructs the image adapter for the given provider.
// model is currently unused (each provider adapter targets a single fixed
// model) but kept in the signature to match NewAdapter's shape for when a
// provider offers multiple image models.
func NewImageAdapter(provider, apiKey, model string) (ImageAdapter, error) {
	switch provider {
	case "openai":
		if apiKey == "" {
			return nil, fmt.Errorf("openai image generation requires an API key")
		}
		return NewOpenAIImageAdapter(apiKey), nil
	default:
		return nil, fmt.Errorf("unknown image provider %q", provider)
	}
}
