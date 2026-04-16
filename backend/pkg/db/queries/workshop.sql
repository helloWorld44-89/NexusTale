-- ============================
-- Workshop Sessions (C2)
-- ============================
-- Named, persistent AI chat sessions per project and user.
-- Messages are stored as a JSONB array; each element is:
--   { "role": "user"|"assistant", "content": "...", "timestamp": "ISO-8601" }

-- name: ListWorkshopSessions :many
SELECT id, project_id, user_id, title, messages, created_at, updated_at
FROM   workshop_sessions
WHERE  project_id = $1
  AND  user_id    = $2
ORDER  BY updated_at DESC;

-- name: CreateWorkshopSession :one
INSERT INTO workshop_sessions (project_id, user_id, title)
VALUES ($1, $2, $3)
RETURNING id, project_id, user_id, title, messages, created_at, updated_at;

-- name: GetWorkshopSession :one
SELECT id, project_id, user_id, title, messages, created_at, updated_at
FROM   workshop_sessions
WHERE  id         = $1
  AND  project_id = $2
  AND  user_id    = $3;

-- name: UpdateWorkshopSession :one
UPDATE workshop_sessions
SET title      = $4,
    messages   = $5,
    updated_at = NOW()
WHERE  id         = $1
  AND  project_id = $2
  AND  user_id    = $3
RETURNING id, project_id, user_id, title, messages, created_at, updated_at;

-- name: DeleteWorkshopSession :exec
DELETE FROM workshop_sessions
WHERE  id         = $1
  AND  project_id = $2
  AND  user_id    = $3;
