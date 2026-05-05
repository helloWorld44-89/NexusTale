CREATE TABLE scene_entity_mentions (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    scene_id    UUID        NOT NULL REFERENCES scenes(id) ON DELETE CASCADE,
    entity_id   UUID        NOT NULL REFERENCES wiki_entities(id) ON DELETE CASCADE,
    project_id  UUID        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    branch_name TEXT        NOT NULL DEFAULT 'canon',
    match_text  TEXT        NOT NULL DEFAULT '',
    suppressed  BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(scene_id, entity_id, branch_name)
);

CREATE INDEX scene_entity_mentions_entity_idx  ON scene_entity_mentions(entity_id, branch_name);
CREATE INDEX scene_entity_mentions_project_idx ON scene_entity_mentions(project_id);

ALTER TABLE projects ADD COLUMN auto_tag_enabled BOOLEAN NOT NULL DEFAULT TRUE;
