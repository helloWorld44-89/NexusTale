-- wiki_entities is the core table for all worldbuilding objects.
-- A single generic table (rather than one per type) keeps queries uniform
-- and makes it easy to add new entity types without schema changes.
-- parent_entity_id enables sub-resources: lore entries are entities whose
-- parent is a location (or any other entity).
CREATE TABLE wiki_entities (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id       UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    parent_entity_id UUID REFERENCES wiki_entities(id) ON DELETE CASCADE,
    type             TEXT NOT NULL CHECK (type IN ('character', 'location', 'faction', 'item', 'concept', 'lore')),
    name             TEXT NOT NULL,
    summary          TEXT NOT NULL DEFAULT '',
    -- attributes stores type-specific fields as freeform JSON so each entity
    -- type can carry its own shape without additional columns or tables.
    attributes       JSONB NOT NULL DEFAULT '{}',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_wiki_entities_project      ON wiki_entities (project_id);
CREATE INDEX idx_wiki_entities_parent       ON wiki_entities (parent_entity_id);
CREATE INDEX idx_wiki_entities_project_type ON wiki_entities (project_id, type);

-- wiki_relationships models directed, typed edges between any two entities.
-- Kept separate from parent_entity_id: parent expresses structural hierarchy
-- (lore belongs to a location), relationships express semantic meaning
-- (character X is allied with faction Y).
CREATE TABLE wiki_relationships (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id     UUID NOT NULL REFERENCES projects(id)      ON DELETE CASCADE,
    from_entity_id UUID NOT NULL REFERENCES wiki_entities(id) ON DELETE CASCADE,
    to_entity_id   UUID NOT NULL REFERENCES wiki_entities(id) ON DELETE CASCADE,
    type           TEXT NOT NULL,
    description    TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_wiki_relationships_project ON wiki_relationships (project_id);
CREATE INDEX idx_wiki_relationships_from    ON wiki_relationships (from_entity_id);
CREATE INDEX idx_wiki_relationships_to      ON wiki_relationships (to_entity_id);

-- wiki_magic_rules stores named constraints for the magic/power system.
-- The description field is designed to be AI-injectable later for consistency checking.
CREATE TABLE wiki_magic_rules (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id  UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_wiki_magic_rules_project ON wiki_magic_rules (project_id);

-- wiki_timeline_events anchors moments to an in-world calendar.
-- year/month/day are nullable to support era-only events ("during the Age of Fire").
-- entity_id optionally associates the event with a specific entity.
CREATE TABLE wiki_timeline_events (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id  UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    entity_id   UUID REFERENCES wiki_entities(id) ON DELETE SET NULL,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    era         TEXT NOT NULL DEFAULT '',
    year        INT,
    month       INT,
    day         INT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_wiki_timeline_project ON wiki_timeline_events (project_id);
