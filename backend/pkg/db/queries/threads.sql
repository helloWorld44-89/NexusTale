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

-- name: CountOpenThreadsByChapter :many
-- Returns, for each chapter in the project, how many open (unresolved) threads
-- were opened within a scene belonging to that chapter.
SELECT s.chapter_id, COUNT(*)::int AS open_thread_count
FROM story_threads st
JOIN scenes s ON s.id = st.opened_at_scene_id
WHERE st.project_id = $1
  AND st.closed_at_scene_id IS NULL
GROUP BY s.chapter_id;

-- name: ListOpenThreadsByProject :many
-- Returns open (unresolved) story threads for a project with their opening chapter title,
-- ordered most-recently-opened first, capped at 10 for context injection.
SELECT st.id, st.project_id, st.title, st.type, st.notes,
       st.opened_at_scene_id, st.closed_at_scene_id, st.created_at, st.updated_at,
       COALESCE(c.title, '') AS chapter_title
FROM story_threads st
LEFT JOIN scenes s  ON s.id  = st.opened_at_scene_id
LEFT JOIN chapters c ON c.id = s.chapter_id
WHERE st.project_id = $1
  AND st.closed_at_scene_id IS NULL
ORDER BY st.created_at DESC
LIMIT 10;
