DROP INDEX IF EXISTS research_notes_embedding_idx;
DROP INDEX IF EXISTS wiki_entities_embedding_idx;
DROP INDEX IF EXISTS chapter_summaries_embedding_idx;

ALTER TABLE research_notes DROP COLUMN IF EXISTS embedding_updated_at, DROP COLUMN IF EXISTS embedding;
ALTER TABLE wiki_entities DROP COLUMN IF EXISTS embedding_updated_at, DROP COLUMN IF EXISTS embedding;
ALTER TABLE chapter_summaries DROP COLUMN IF EXISTS embedding_updated_at, DROP COLUMN IF EXISTS embedding;
