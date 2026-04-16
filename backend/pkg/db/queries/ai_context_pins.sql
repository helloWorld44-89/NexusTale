-- ========================
-- AI Context Pins (C2)
-- ========================
-- Per-user, per-project pins that are injected into the AI context window on
-- every call. Writers can pin wiki entities, chapters, or scenes to ensure
-- Nexus always has a specific piece of information in scope.

-- name: ListContextPins :many
SELECT *
FROM   ai_context_pins
WHERE  project_id = $1
  AND  user_id    = $2
ORDER  BY sort_order ASC, created_at ASC;

-- name: CreateContextPin :one
INSERT INTO ai_context_pins (project_id, user_id, pin_type, ref_id, include_mode)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (project_id, user_id, pin_type, ref_id)
DO UPDATE SET include_mode = EXCLUDED.include_mode
RETURNING *;

-- name: DeleteContextPin :exec
DELETE FROM ai_context_pins
WHERE id = $1 AND project_id = $2 AND user_id = $3;
