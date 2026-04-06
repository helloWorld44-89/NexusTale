CREATE TABLE forks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    source_project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    forked_project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    branch_name TEXT NOT NULL,
    source_commit_sha TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (source_project_id, forked_project_id)
);

CREATE INDEX idx_forks_source ON forks (source_project_id);
CREATE INDEX idx_forks_forked ON forks (forked_project_id);
