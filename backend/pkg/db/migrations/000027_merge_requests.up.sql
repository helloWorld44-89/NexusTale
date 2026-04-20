CREATE TABLE merge_requests (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id    UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  from_branch   TEXT NOT NULL,
  to_branch     TEXT NOT NULL DEFAULT 'canon',
  title         TEXT NOT NULL,
  description   TEXT NOT NULL DEFAULT '',
  requested_by  UUID NOT NULL REFERENCES users(id),
  status        TEXT NOT NULL DEFAULT 'open'
                CHECK (status IN ('open','approved','rejected','merged')),
  reviewer_note TEXT NOT NULL DEFAULT '',
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  resolved_at   TIMESTAMPTZ
);

CREATE INDEX ON merge_requests (project_id, status);
CREATE INDEX ON merge_requests (requested_by);
