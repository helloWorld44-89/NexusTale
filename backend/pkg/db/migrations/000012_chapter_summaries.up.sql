-- chapter_summaries: branch-isolated AI-generated chapter summaries.
-- PK is (chapter_id, branch_name) so diverged timelines never share a summary row.
CREATE TABLE chapter_summaries (
    chapter_id  UUID        NOT NULL REFERENCES chapters(id) ON DELETE CASCADE,
    branch_name TEXT        NOT NULL DEFAULT 'canon',
    ai_summary  TEXT        NOT NULL DEFAULT '',
    stale       BOOLEAN     NOT NULL DEFAULT FALSE,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (chapter_id, branch_name)
);

-- project_active_branch: tracks which git Timeline each user is currently on,
-- per project. Defaults to 'canon'. Updated on every TravelTo / Diverge.
CREATE TABLE project_active_branch (
    project_id  UUID        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id     UUID        NOT NULL REFERENCES users(id)    ON DELETE CASCADE,
    branch_name TEXT        NOT NULL DEFAULT 'canon',
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (project_id, user_id)
);
