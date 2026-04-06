package adapters

// Anthropic adapter: wraps the /v1/messages endpoint.
// All Claude models support streaming — no thinking-model special-casing required.
// Handles anthropic-version header, SSE delta parsing, and token counting.
