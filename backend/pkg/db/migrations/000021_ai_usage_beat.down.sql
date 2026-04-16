DROP INDEX IF EXISTS idx_ai_usage_beat;
ALTER TABLE ai_usage
    DROP COLUMN IF EXISTS scene_id,
    DROP COLUMN IF EXISTS beat_text,
    DROP COLUMN IF EXISTS mode;
