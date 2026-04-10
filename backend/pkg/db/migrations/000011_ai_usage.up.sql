-- Per-call token and cost tracking for AI operations.
-- Recorded non-blocking after every adapter call; used for ProjectHome usage stats.
CREATE TABLE ai_usage (
    id                UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id           UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    project_id        UUID        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    model             TEXT        NOT NULL DEFAULT '',
    prompt_tokens     INT         NOT NULL DEFAULT 0,
    completion_tokens INT         NOT NULL DEFAULT 0,
    cost_usd          NUMERIC(12,8) NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_ai_usage_project_id ON ai_usage (project_id);
CREATE INDEX idx_ai_usage_user_id    ON ai_usage (user_id);
CREATE INDEX idx_ai_usage_created_at ON ai_usage (created_at);
