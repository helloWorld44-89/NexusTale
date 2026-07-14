-- migration 037: semantic embeddings for RAG
-- Requires the pgvector extension (available in pgvector/pgvector:pg16 image).
-- Dimension 768 matches text-embedding-3-small (with dimensions=768),
-- nomic-embed-text (Ollama), and text-embedding-004 (Gemini).

CREATE EXTENSION IF NOT EXISTS vector;

ALTER TABLE chapter_summaries
  ADD COLUMN IF NOT EXISTS embedding vector(768),
  ADD COLUMN IF NOT EXISTS embedding_updated_at TIMESTAMPTZ;

ALTER TABLE wiki_entities
  ADD COLUMN IF NOT EXISTS embedding vector(768),
  ADD COLUMN IF NOT EXISTS embedding_updated_at TIMESTAMPTZ;

ALTER TABLE research_notes
  ADD COLUMN IF NOT EXISTS embedding vector(768),
  ADD COLUMN IF NOT EXISTS embedding_updated_at TIMESTAMPTZ;

-- IVFFlat indexes for approximate nearest-neighbour search.
-- lists=10 is appropriate for datasets up to ~10k rows per project.
CREATE INDEX IF NOT EXISTS chapter_summaries_embedding_idx
  ON chapter_summaries USING ivfflat (embedding vector_cosine_ops)
  WITH (lists = 10);

CREATE INDEX IF NOT EXISTS wiki_entities_embedding_idx
  ON wiki_entities USING ivfflat (embedding vector_cosine_ops)
  WITH (lists = 10);

CREATE INDEX IF NOT EXISTS research_notes_embedding_idx
  ON research_notes USING ivfflat (embedding vector_cosine_ops)
  WITH (lists = 10);
