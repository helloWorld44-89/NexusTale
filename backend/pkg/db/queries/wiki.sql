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

-- name: GetEntitiesByNames :many
SELECT * FROM wiki_entities
WHERE project_id = sqlc.arg('project_id')
  AND LOWER(name) = ANY(sqlc.arg('names')::text[])
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

-- name: UpdateEntityImage :one
UPDATE wiki_entities
SET image_key  = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ClearEntityImage :one
UPDATE wiki_entities
SET image_key  = NULL,
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
INSERT INTO wiki_magic_rules (project_id, name, description, attributes)
VALUES ($1, $2, $3, $4)
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
    attributes  = COALESCE(sqlc.narg('attributes'), attributes),
    updated_at  = now()
WHERE id = $1
RETURNING *;

-- name: DeleteMagicRule :exec
DELETE FROM wiki_magic_rules WHERE id = $1;

-- name: ListMagicRulesForContext :many
-- Returns the 5 most recently updated magic rules for AI context injection.
SELECT id, project_id, name, description, attributes, updated_at
FROM wiki_magic_rules
WHERE project_id = $1
ORDER BY updated_at DESC
LIMIT 5;

-- ========================
-- Timeline Events
-- ========================

-- name: CreateTimelineEvent :one
INSERT INTO wiki_timeline_events (
    project_id, entity_id, name, description, era,
    year, month, day,
    anchor_event_id, anchor_offset_year, anchor_offset_month, anchor_offset_day
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: GetTimelineEvent :one
SELECT * FROM wiki_timeline_events WHERE id = $1;

-- name: ListTimelineEventsByProject :many
SELECT * FROM wiki_timeline_events
WHERE project_id = $1
ORDER BY year NULLS LAST, month NULLS LAST, day NULLS LAST, name ASC;

-- name: UpdateTimelineEvent :one
UPDATE wiki_timeline_events
SET name                = COALESCE(sqlc.narg('name'), name),
    description         = COALESCE(sqlc.narg('description'), description),
    era                 = COALESCE(sqlc.narg('era'), era),
    year                = COALESCE(sqlc.narg('year'), year),
    month               = COALESCE(sqlc.narg('month'), month),
    day                 = COALESCE(sqlc.narg('day'), day),
    anchor_event_id     = COALESCE(sqlc.narg('anchor_event_id'), anchor_event_id),
    anchor_offset_year  = COALESCE(sqlc.narg('anchor_offset_year'), anchor_offset_year),
    anchor_offset_month = COALESCE(sqlc.narg('anchor_offset_month'), anchor_offset_month),
    anchor_offset_day   = COALESCE(sqlc.narg('anchor_offset_day'), anchor_offset_day),
    updated_at          = now()
WHERE id = $1
RETURNING *;

-- name: DeleteTimelineEvent :exec
DELETE FROM wiki_timeline_events WHERE id = $1;

-- ========================
-- Scene entity mentions
-- ========================

-- name: DeleteSceneEntityMentions :exec
DELETE FROM scene_entity_mentions
WHERE scene_id = $1 AND branch_name = $2;

-- name: UpsertSceneEntityMention :one
INSERT INTO scene_entity_mentions (scene_id, entity_id, project_id, branch_name, match_text)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (scene_id, entity_id, branch_name)
DO UPDATE SET match_text = EXCLUDED.match_text, suppressed = FALSE
RETURNING *;

-- name: ListMentionsByScene :many
SELECT sem.id, sem.scene_id, sem.entity_id, sem.project_id, sem.branch_name,
       sem.match_text, sem.suppressed, sem.created_at,
       we.name AS entity_name, we.type AS entity_type
FROM scene_entity_mentions sem
JOIN wiki_entities we ON we.id = sem.entity_id
WHERE sem.scene_id = $1 AND sem.branch_name = $2 AND sem.suppressed = FALSE
ORDER BY we.name ASC;

-- name: SuppressMention :exec
UPDATE scene_entity_mentions SET suppressed = TRUE WHERE id = $1;

-- name: SuppressAllMentions :exec
UPDATE scene_entity_mentions SET suppressed = TRUE
WHERE scene_id = $1 AND branch_name = $2;

-- name: ListScenesByEntity :many
SELECT
    s.id         AS scene_id,
    s.title      AS scene_title,
    s.sort_order AS scene_order,
    c.id         AS chapter_id,
    c.title      AS chapter_title,
    c.sort_order AS chapter_order,
    sem.branch_name
FROM scenes s
JOIN scene_entity_mentions sem ON sem.scene_id = s.id
JOIN chapters c ON c.id = s.chapter_id
WHERE sem.entity_id = $1 AND sem.branch_name = $2 AND sem.suppressed = FALSE
ORDER BY c.sort_order, s.sort_order;

-- name: ListMentionedEntitiesByScene :many
SELECT we.*
FROM wiki_entities we
JOIN scene_entity_mentions sem ON sem.entity_id = we.id
WHERE sem.scene_id = sqlc.arg('scene_id')::uuid
  AND sem.branch_name = sqlc.arg('branch_name')
  AND sem.suppressed = FALSE
ORDER BY we.name ASC;
