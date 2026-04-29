-- name: CreateScene :one
INSERT INTO scenes (chapter_id, title, pov, tense, tags, summary, sort_order, word_count)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, chapter_id, title, pov, tense, tags, summary, summary_stale, sort_order, created_at, updated_at, word_count;

-- name: GetScene :one
SELECT id, chapter_id, title, pov, tense, tags, summary, summary_stale, sort_order, created_at, updated_at, word_count
FROM scenes
WHERE id = $1;

-- name: ListScenesByChapter :many
SELECT id, chapter_id, title, pov, tense, tags, summary, summary_stale, sort_order, created_at, updated_at, word_count
FROM scenes
WHERE chapter_id = $1
ORDER BY sort_order ASC;

-- name: UpdateScene :one
UPDATE scenes
SET title = COALESCE(sqlc.narg('title'), title),
    pov = COALESCE(sqlc.narg('pov'), pov),
    tense = COALESCE(sqlc.narg('tense'), tense),
    tags = COALESCE(sqlc.narg('tags'), tags),
    summary = COALESCE(sqlc.narg('summary'), summary),
    summary_stale = COALESCE(sqlc.narg('summary_stale'), summary_stale),
    sort_order = COALESCE(sqlc.narg('sort_order'), sort_order),
    word_count = COALESCE(sqlc.narg('word_count'), word_count),
    updated_at = now()
WHERE id = $1
RETURNING id, chapter_id, title, pov, tense, tags, summary, summary_stale, sort_order, created_at, updated_at, word_count;

-- name: DeleteScene :exec
DELETE FROM scenes WHERE id = $1;
