-- name: ListProjectPrompts :many
SELECT id, project_id, name, category, content, system_content, sort_order, created_at, updated_at
FROM project_prompts
WHERE project_id = $1
ORDER BY sort_order ASC, created_at ASC;

-- name: GetProjectPrompt :one
SELECT id, project_id, name, category, content, system_content, sort_order, created_at, updated_at
FROM project_prompts
WHERE id = $1;

-- name: CreateProjectPrompt :one
INSERT INTO project_prompts (project_id, name, category, content, system_content, sort_order)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, project_id, name, category, content, system_content, sort_order, created_at, updated_at;

-- name: UpdateProjectPrompt :one
UPDATE project_prompts
SET name           = COALESCE(sqlc.narg('name'), name),
    category       = COALESCE(sqlc.narg('category'), category),
    content        = COALESCE(sqlc.narg('content'), content),
    system_content = COALESCE(sqlc.narg('system_content'), system_content),
    sort_order     = COALESCE(sqlc.narg('sort_order'), sort_order),
    updated_at     = now()
WHERE id = $1
RETURNING id, project_id, name, category, content, system_content, sort_order, created_at, updated_at;

-- name: DeleteProjectPrompt :exec
DELETE FROM project_prompts
WHERE id = $1;
