-- ============================
-- Research Notes (C2)
-- ============================
-- Per-project scratchpad entries for web quotes, worldbuilding facts,
-- craft references, etc.  Notes are project-wide (not per-user) so all
-- collaborators on a project share the same pool.

-- name: ListResearchNotes :many
SELECT id, project_id, user_id, title, body, source_url, tags, created_at, updated_at
FROM   research_notes
WHERE  project_id = $1
ORDER  BY updated_at DESC;

-- name: CreateResearchNote :one
INSERT INTO research_notes (project_id, user_id, title, body, source_url, tags)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, project_id, user_id, title, body, source_url, tags, created_at, updated_at;

-- name: GetResearchNote :one
-- Project-scoped fetch; used by CRUD handlers.
SELECT id, project_id, user_id, title, body, source_url, tags, created_at, updated_at
FROM   research_notes
WHERE  id         = $1
  AND  project_id = $2;

-- name: GetResearchNoteByID :one
-- ID-only fetch; used by the AI context builder (no project_id needed — UUIDs are unguessable).
SELECT id, project_id, user_id, title, body, source_url, tags, created_at, updated_at
FROM   research_notes
WHERE  id = $1;

-- name: UpdateResearchNote :one
UPDATE research_notes
SET title      = $3,
    body       = $4,
    source_url = $5,
    tags       = $6,
    updated_at = NOW()
WHERE  id         = $1
  AND  project_id = $2
RETURNING id, project_id, user_id, title, body, source_url, tags, created_at, updated_at;

-- name: DeleteResearchNote :exec
DELETE FROM research_notes
WHERE  id         = $1
  AND  project_id = $2;
