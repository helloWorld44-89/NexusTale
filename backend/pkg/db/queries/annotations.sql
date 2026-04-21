-- ============================
-- Manuscript Annotations (C3.4)
-- ============================
-- Per-scene inline notes, suggestions, and questions left by reviewers/editors.
-- Resolved by the project owner only.

-- name: ListAnnotationsByScene :many
SELECT
    a.id, a.project_id, a.scene_id, a.author_id,
    u.display_name  AS author_name,
    a.start_char, a.end_char, a.body, a.type,
    a.resolved, a.resolved_by, a.created_at
FROM   manuscript_annotations a
JOIN   users u ON u.id = a.author_id
WHERE  a.scene_id = $1
ORDER  BY a.start_char ASC;

-- name: CreateAnnotation :one
INSERT INTO manuscript_annotations
    (project_id, scene_id, author_id, start_char, end_char, body, type)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, project_id, scene_id, author_id, start_char, end_char, body, type, resolved, resolved_by, created_at;

-- name: GetAnnotation :one
SELECT id, project_id, scene_id, author_id, start_char, end_char, body, type, resolved, resolved_by, created_at
FROM   manuscript_annotations
WHERE  id = $1 AND project_id = $2;

-- name: UpdateAnnotationBody :one
UPDATE manuscript_annotations
SET    body = $3
WHERE  id = $1 AND project_id = $2
RETURNING id, project_id, scene_id, author_id, start_char, end_char, body, type, resolved, resolved_by, created_at;

-- name: ResolveAnnotation :one
UPDATE manuscript_annotations
SET    resolved = true, resolved_by = $3
WHERE  id = $1 AND project_id = $2
RETURNING id, project_id, scene_id, author_id, start_char, end_char, body, type, resolved, resolved_by, created_at;

-- name: DeleteAnnotation :exec
DELETE FROM manuscript_annotations
WHERE  id = $1 AND project_id = $2;
