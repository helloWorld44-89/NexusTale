-- name: CreateAct :one
INSERT INTO acts (project_id, title, summary, sort_order)
VALUES ($1, $2, $3, $4)
RETURNING id, project_id, title, summary, sort_order, created_at, updated_at;

-- name: GetAct :one
SELECT id, project_id, title, summary, sort_order, created_at, updated_at
FROM acts
WHERE id = $1;

-- name: ListActsByProject :many
SELECT id, project_id, title, summary, sort_order, created_at, updated_at
FROM acts
WHERE project_id = $1
ORDER BY sort_order ASC;

-- name: UpdateAct :one
UPDATE acts
SET title      = COALESCE(sqlc.narg('title'), title),
    summary    = COALESCE(sqlc.narg('summary'), summary),
    sort_order = COALESCE(sqlc.narg('sort_order'), sort_order),
    updated_at = now()
WHERE id = $1
RETURNING id, project_id, title, summary, sort_order, created_at, updated_at;

-- name: DeleteAct :exec
DELETE FROM acts WHERE id = $1;
