-- export_jobs: tracks async EPUB export requests.
-- Markdown exports are synchronous and never create rows here.
CREATE TABLE export_jobs (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id  UUID        NOT NULL REFERENCES projects(id)  ON DELETE CASCADE,
    user_id     UUID        NOT NULL REFERENCES users(id)     ON DELETE CASCADE,
    format      TEXT        NOT NULL CHECK (format IN ('markdown', 'epub')),
    status      TEXT        NOT NULL DEFAULT 'pending'
                            CHECK (status IN ('pending', 'processing', 'done', 'failed')),
    minio_key   TEXT,       -- set when status = done
    error_msg   TEXT,       -- set when status = failed
    expires_at  TIMESTAMPTZ,-- set when status = done; after this the MinIO object may be gone
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX export_jobs_project_id_idx ON export_jobs(project_id);
CREATE INDEX export_jobs_status_idx     ON export_jobs(status) WHERE status IN ('pending', 'processing');
