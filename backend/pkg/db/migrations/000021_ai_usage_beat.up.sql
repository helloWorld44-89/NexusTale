-- Extend ai_usage with per-call mode, beat text, and scene context.
-- mode:      'beat' | 'continue' | 'chat' | 'summarize' — which AI surface triggered the call.
-- beat_text: the beat sentence typed by the writer (mode=beat only; empty otherwise).
-- scene_id:  scene in focus at call time (NULL when not applicable).
ALTER TABLE ai_usage
    ADD COLUMN mode      TEXT NOT NULL DEFAULT '',
    ADD COLUMN beat_text TEXT NOT NULL DEFAULT '',
    ADD COLUMN scene_id  UUID REFERENCES scenes(id) ON DELETE SET NULL;

CREATE INDEX idx_ai_usage_beat ON ai_usage (project_id, mode, created_at DESC)
    WHERE mode = 'beat' AND beat_text != '';
