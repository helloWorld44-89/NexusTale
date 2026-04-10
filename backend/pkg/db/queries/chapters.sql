-- name: CreateChapter :one
INSERT INTO chapters (project_id, act_id, title, summary, sort_order)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, project_id, title, summary, sort_order, created_at, updated_at, act_id;

-- name: GetChapter :one
SELECT id, project_id, title, summary, sort_order, created_at, updated_at, act_id
FROM chapters
WHERE id = $1;

-- name: ListChaptersByAct :many
SELECT id, project_id, title, summary, sort_order, created_at, updated_at, act_id
FROM chapters
WHERE act_id = $1
ORDER BY sort_order ASC;

-- name: ListChaptersByProject :many
SELECT id, project_id, title, summary, sort_order, created_at, updated_at, act_id
FROM chapters
WHERE project_id = $1
ORDER BY sort_order ASC;

-- name: UpdateChapter :one
UPDATE chapters
SET title      = COALESCE(sqlc.narg('title'), title),
    summary    = COALESCE(sqlc.narg('summary'), summary),
    sort_order = COALESCE(sqlc.narg('sort_order'), sort_order),
    updated_at = now()
WHERE id = $1
RETURNING id, project_id, title, summary, sort_order, created_at, updated_at, act_id;

-- name: DeleteChapter :exec
DELETE FROM chapters WHERE id = $1;
