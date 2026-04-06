package ai

// Embeddings manages the pgvector store for semantic search over wiki entities
// and scene summaries. Used by the prompt engine for RAG context injection.
// Embed generates and upserts a vector for a given text chunk with project scope.
// Search returns the top-K most relevant chunks for a query, scoped to a project.
