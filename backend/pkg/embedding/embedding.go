package embedding

import (
	"context"
	"fmt"
	"strings"
)

// Dims is the fixed vector dimension used across all embedding providers.
// 768 is supported natively by nomic-embed-text (Ollama), text-embedding-004
// (Gemini), and by text-embedding-3-small with the dimensions=768 parameter
// (OpenAI). Keeping one dimension avoids mixed-column issues in pgvector.
const Dims = 768

// Embedder generates a dense vector representation for a text string.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	Provider() string
}

// VecToString encodes a float32 slice as the PostgreSQL vector literal used
// when passing vectors to pgvector via pgx without the pgvector-go library.
// Example: [0.1, -0.3, 0.9]
func VecToString(v []float32) string {
	parts := make([]string, len(v))
	for i, f := range v {
		parts[i] = fmt.Sprintf("%g", f)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
