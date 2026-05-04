-- name: CreateThread :one
INSERT INTO story_threads (project_id, title, type, notes, opened_at_scene_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListThreadsByProject :many
SELECT * FROM story_threads
WHERE project_id = $1
ORDER BY created_at ASC;

-- name: GetThread :one
SELECT * FROM story_threads WHERE id = $1;

-- name: UpdateThread :one
UPDATE story_threads
SET title               = COALESCE(sqlc.narg('title'), title),
    type                = COALESCE(sqlc.narg('type'), type),
    notes               = COALESCE(sqlc.narg('notes'), notes),
    opened_at_scene_id  = COALESCE(sqlc.narg('opened_at_scene_id'), opened_at_scene_id),
    closed_at_scene_id  = sqlc.narg('closed_at_scene_id'),
    updated_at          = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteThread :exec
DELETE FROM story_threads WHERE id = $1;
