-- workshop_sessions stores named, persistent AI chat sessions per project.
-- Each session belongs to one user and holds its full message history as JSONB
-- so it survives page reloads and can be exported to Markdown.

CREATE TABLE workshop_sessions (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id  UUID        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id     UUID        NOT NULL REFERENCES users(id)    ON DELETE CASCADE,
    title       TEXT        NOT NULL DEFAULT 'New Session',
    messages    JSONB       NOT NULL DEFAULT '[]',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX ON workshop_sessions(project_id, user_id, updated_at DESC);
