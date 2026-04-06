package adapters

// OpenRouter adapter: wraps openrouter.ai/api/v1/chat/completions.
// Passes HTTP-Referer and X-Title headers per OpenRouter's usage policy.
// Shares the thinking-model detection logic with the OpenAI adapter.
