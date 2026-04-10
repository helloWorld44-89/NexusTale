-- Acts sit between projects and chapters in the hierarchy.
-- Acts are required: every chapter must belong to an act.
-- A default "Act 1" is created for each existing project so chapters are never orphaned.

CREATE TABLE acts (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title      TEXT NOT NULL DEFAULT 'Act 1',
    summary    TEXT NOT NULL DEFAULT '',
    sort_order INT  NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_acts_project_id ON acts (project_id);

-- Backfill: one default act per existing project.
INSERT INTO acts (project_id, title, sort_order)
SELECT id, 'Act 1', 0 FROM projects;

-- Wire chapters to acts. Keep project_id denormalized for single-query project scans.
ALTER TABLE chapters
    ADD COLUMN act_id UUID REFERENCES acts(id) ON DELETE CASCADE;

-- Assign every existing chapter to its project's default act.
UPDATE chapters c
SET act_id = (
    SELECT a.id FROM acts a WHERE a.project_id = c.project_id ORDER BY a.sort_order ASC LIMIT 1
);

-- Now that all rows are backfilled, enforce NOT NULL.
ALTER TABLE chapters ALTER COLUMN act_id SET NOT NULL;

CREATE INDEX idx_chapters_act_id ON chapters (act_id);
