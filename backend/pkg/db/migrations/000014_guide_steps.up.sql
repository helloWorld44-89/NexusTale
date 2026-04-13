-- guide_steps: tracks per-project wizard progress.
-- Each row is one step; data holds the user's answers as JSONB.
-- completed_at is NULL until the user completes (not just saves) the step.
CREATE TABLE guide_steps (
    project_id   UUID        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    step_key     TEXT        NOT NULL CHECK (step_key IN ('premise', 'characters', 'world', 'outline', 'first_scene')),
    data         JSONB       NOT NULL DEFAULT '{}',
    completed_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (project_id, step_key)
);

CREATE INDEX guide_steps_project_id_idx ON guide_steps(project_id);
