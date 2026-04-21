CREATE TABLE manuscript_annotations (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id  UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  scene_id    UUID NOT NULL REFERENCES scenes(id) ON DELETE CASCADE,
  author_id   UUID NOT NULL REFERENCES users(id),
  start_char  INT NOT NULL,
  end_char    INT NOT NULL,
  body        TEXT NOT NULL,
  type        TEXT NOT NULL DEFAULT 'note'
              CHECK (type IN ('note','suggestion','question')),
  resolved    BOOLEAN NOT NULL DEFAULT false,
  resolved_by UUID REFERENCES users(id),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ON manuscript_annotations(scene_id);
CREATE INDEX ON manuscript_annotations(project_id);
