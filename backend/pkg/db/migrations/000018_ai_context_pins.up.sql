CREATE TABLE ai_context_pins (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id    UUID        NOT NULL REFERENCES users(id)    ON DELETE CASCADE,
    -- pin_type: what kind of object is pinned
    pin_type   TEXT        NOT NULL CHECK (pin_type IN ('entity', 'chapter', 'scene')),
    -- ref_id: UUID of the entity / chapter / scene being pinned
    ref_id     UUID        NOT NULL,
    -- include_mode: 'summary' uses AI summary or first snippet; 'full' uses complete content
    include_mode TEXT      NOT NULL DEFAULT 'summary' CHECK (include_mode IN ('summary', 'full')),
    sort_order  INT        NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, user_id, pin_type, ref_id)
);

CREATE INDEX idx_ai_context_pins_project_user ON ai_context_pins (project_id, user_id);
