CREATE TABLE notifications (
  id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  project_id UUID        REFERENCES projects(id) ON DELETE CASCADE,
  type       TEXT        NOT NULL,
  payload    JSONB       NOT NULL DEFAULT '{}',
  read_at    TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Efficient unread count + inbox query without full-table scans.
CREATE INDEX ON notifications (user_id, read_at) WHERE read_at IS NULL;
CREATE INDEX ON notifications (user_id, created_at DESC);
