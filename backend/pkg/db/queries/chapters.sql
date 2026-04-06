-- name: CreateChapter :one
INSERT INTO chapters (project_id, title, summary, sort_order)
VALUES ($1, $2, $3, $4)
RETURNING id, project_id, title, summary, sort_order, created_at, updated_at;

-- name: GetChapter :one
SELECT id, project_id, title, summary, sort_order, created_at, updated_at
FROM chapters
WHERE id = $1;

-- name: ListChaptersByProject :many
SELECT id, project_id, title, summary, sort_order, created_at, updated_at
FROM chapters
WHERE project_id = $1
ORDER BY sort_order ASC;

-- name: UpdateChapter :one
UPDATE chapters
SET title = COALESCE(sqlc.narg('title'), title),
    summary = COALESCE(sqlc.narg('summary'), summary),
    sort_order = COALESCE(sqlc.narg('sort_order'), sort_order),
    updated_at = now()
WHERE id = $1
RETURNING id, project_id, title, summary, sort_order, created_at, updated_at;

-- name: DeleteChapter :exec
DELETE FROM chapters WHERE id = $1;
