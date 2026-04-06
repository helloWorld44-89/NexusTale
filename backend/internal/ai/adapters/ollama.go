package adapters

// Ollama adapter: speaks to a local Ollama server (default :11434).
// Also compatible with llama-server's /completion endpoint
// (matches Writingway2's streamGenerationLocal target).
// Converts the messages array to ChatML format for models that require it
// using the same <|im_start|>/<|im_end|> template as Writingway2.
