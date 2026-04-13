-- name: UpsertGuideStep :one
INSERT INTO guide_steps (project_id, step_key, data, updated_at)
VALUES ($1, $2, $3, NOW())
ON CONFLICT (project_id, step_key) DO UPDATE
    SET data       = EXCLUDED.data,
        updated_at = NOW()
RETURNING project_id, step_key, data, completed_at, created_at, updated_at;

-- name: CompleteGuideStep :one
UPDATE guide_steps
SET completed_at = NOW(),
    data         = $3,
    updated_at   = NOW()
WHERE project_id = $1
  AND step_key   = $2
RETURNING project_id, step_key, data, completed_at, created_at, updated_at;

-- name: GetGuideStep :one
SELECT project_id, step_key, data, completed_at, created_at, updated_at
FROM guide_steps
WHERE project_id = $1
  AND step_key   = $2;

-- name: ListGuideSteps :many
SELECT project_id, step_key, data, completed_at, created_at, updated_at
FROM guide_steps
WHERE project_id = $1
ORDER BY CASE step_key
    WHEN 'premise'     THEN 1
    WHEN 'characters'  THEN 2
    WHEN 'world'       THEN 3
    WHEN 'outline'     THEN 4
    WHEN 'first_scene' THEN 5
    ELSE 99
END;
