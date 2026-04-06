package ai

// Proxy selects the correct adapter (remote API vs local Ollama) based on
// per-project model config and routes the assembled prompt to it.
// It also applies token-budget limits, injects RAG context from pgvector,
// and streams the response back to the client via SSE.
