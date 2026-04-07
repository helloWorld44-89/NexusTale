-- ========================
-- Entities
-- ========================

-- name: CreateEntity :one
INSERT INTO wiki_entities (project_id, parent_entity_id, type, name, summary, attributes)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetEntity :one
SELECT * FROM wiki_entities WHERE id = $1;

-- name: ListEntitiesByProject :many
SELECT * FROM wiki_entities
WHERE project_id = $1
  AND (sqlc.narg('type')::text IS NULL OR type = sqlc.narg('type')::text)
ORDER BY name ASC;

-- name: ListEntitiesByParent :many
SELECT * FROM wiki_entities
WHERE parent_entity_id = $1
ORDER BY name ASC;

-- name: UpdateEntity :one
UPDATE wiki_entities
SET name       = COALESCE(sqlc.narg('name'), name),
    summary    = COALESCE(sqlc.narg('summary'), summary),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateEntityAttributes :one
UPDATE wiki_entities
SET attributes = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteEntity :exec
DELETE FROM wiki_entities WHERE id = $1;

-- ========================
-- Relationships
-- ========================

-- name: CreateRelationship :one
INSERT INTO wiki_relationships (project_id, from_entity_id, to_entity_id, type, description)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetRelationship :one
SELECT * FROM wiki_relationships WHERE id = $1;

-- name: ListRelationshipsByProject :many
SELECT * FROM wiki_relationships
WHERE project_id = $1
ORDER BY created_at ASC;

-- name: DeleteRelationship :exec
DELETE FROM wiki_relationships WHERE id = $1;

-- ========================
-- Magic Rules
-- ========================

-- name: CreateMagicRule :one
INSERT INTO wiki_magic_rules (project_id, name, description)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetMagicRule :one
SELECT * FROM wiki_magic_rules WHERE id = $1;

-- name: ListMagicRulesByProject :many
SELECT * FROM wiki_magic_rules
WHERE project_id = $1
ORDER BY name ASC;

-- name: UpdateMagicRule :one
UPDATE wiki_magic_rules
SET name        = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    updated_at  = now()
WHERE id = $1
RETURNING *;

-- name: DeleteMagicRule :exec
DELETE FROM wiki_magic_rules WHERE id = $1;

-- ========================
-- Timeline Events
-- ========================

-- name: CreateTimelineEvent :one
INSERT INTO wiki_timeline_events (project_id, entity_id, name, description, era, year, month, day)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetTimelineEvent :one
SELECT * FROM wiki_timeline_events WHERE id = $1;

-- name: ListTimelineEventsByProject :many
SELECT * FROM wiki_timeline_events
WHERE project_id = $1
ORDER BY year NULLS LAST, month NULLS LAST, day NULLS LAST, name ASC;

-- name: UpdateTimelineEvent :one
UPDATE wiki_timeline_events
SET name        = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    era         = COALESCE(sqlc.narg('era'), era),
    year        = COALESCE(sqlc.narg('year'), year),
    month       = COALESCE(sqlc.narg('month'), month),
    day         = COALESCE(sqlc.narg('day'), day),
    updated_at  = now()
WHERE id = $1
RETURNING *;

-- name: DeleteTimelineEvent :exec
DELETE FROM wiki_timeline_events WHERE id = $1;
