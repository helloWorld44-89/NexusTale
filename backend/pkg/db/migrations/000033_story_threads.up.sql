CREATE TABLE story_threads (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id          UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title               TEXT NOT NULL,
    type                TEXT NOT NULL,  -- world | mystery | arc | conflict
    notes               TEXT NOT NULL DEFAULT '',
    opened_at_scene_id  UUID REFERENCES scenes(id) ON DELETE SET NULL,
    closed_at_scene_id  UUID REFERENCES scenes(id) ON DELETE SET NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_story_threads_project ON story_threads(project_id);
