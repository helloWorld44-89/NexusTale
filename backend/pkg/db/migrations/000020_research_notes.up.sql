-- research_notes stores freeform per-project scratchpad entries.
-- Writers paste web quotes, worldbuilding facts, craft references, etc.
-- Notes can be pinned into the AI context window via the Context Pins panel.

CREATE TABLE research_notes (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id  UUID        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id     UUID        NOT NULL REFERENCES users(id)    ON DELETE CASCADE,
    title       TEXT        NOT NULL DEFAULT 'Untitled Note',
    body        TEXT        NOT NULL DEFAULT '',
    source_url  TEXT        NOT NULL DEFAULT '',
    tags        TEXT[]      NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX ON research_notes(project_id, updated_at DESC);
