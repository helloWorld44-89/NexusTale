package adapters

// OpenAI adapter: wraps the OpenAI chat completions endpoint.
// Handles streaming SSE, thinking-model detection (o-series: o1, o3, o4...),
// and automatic fallback to non-streaming for reasoning models.
// Mirrors the isThinkingModel detection logic from Writingway2's generation.js.
