-- Writing style presets per project.
-- Writers can create named prompts that override the system prompt and/or inject
-- a style instruction block into the user turn at AI generation time.
CREATE TABLE project_prompts (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id     UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name           TEXT NOT NULL,
    category       TEXT NOT NULL DEFAULT 'prose',  -- 'prose' | 'workshop'
    content        TEXT NOT NULL DEFAULT '',        -- style instruction appended to user turn
    system_content TEXT NOT NULL DEFAULT '',        -- replaces default system prompt when non-empty
    sort_order     INT  NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_project_prompts_project_id ON project_prompts (project_id);

-- Allow operators to force batch+simulated streaming for unreliable provider keys.
ALTER TABLE user_api_keys
    ADD COLUMN force_non_streaming BOOL NOT NULL DEFAULT false;
