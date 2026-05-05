-- name: CreateProject :one
INSERT INTO projects (owner_id, title, description, genres, git_repo_path)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, owner_id, title, description, genres, git_repo_path, archived, created_at, updated_at, structure_id, structure_custom, ai_instructions, phase, auto_tag_enabled;

-- name: GetProject :one
SELECT id, owner_id, title, description, genres, git_repo_path, archived, created_at, updated_at, structure_id, structure_custom, ai_instructions, phase, auto_tag_enabled
FROM projects
WHERE id = $1;

-- name: ListProjectsByOwner :many
SELECT id, owner_id, title, description, genres, git_repo_path, archived, created_at, updated_at, structure_id, structure_custom, ai_instructions, phase, auto_tag_enabled
FROM projects
WHERE owner_id = $1 AND archived = false
ORDER BY updated_at DESC;

-- name: ListProjectsForUser :many
SELECT DISTINCT p.id, p.owner_id, p.title, p.description, p.genres, p.git_repo_path,
       p.archived, p.created_at, p.updated_at, p.structure_id, p.structure_custom, p.ai_instructions, p.phase, p.auto_tag_enabled
FROM projects p
LEFT JOIN project_collaborators pc ON pc.project_id = p.id AND pc.user_id = $1
WHERE p.archived = false
  AND (p.owner_id = $1 OR pc.user_id IS NOT NULL)
ORDER BY p.updated_at DESC;

-- name: UpdateProject :one
UPDATE projects
SET title            = COALESCE(sqlc.narg('title'), title),
    description      = COALESCE(sqlc.narg('description'), description),
    auto_tag_enabled = COALESCE(sqlc.narg('auto_tag_enabled'), auto_tag_enabled),
    updated_at       = now()
WHERE id = $1
RETURNING id, owner_id, title, description, genres, git_repo_path, archived, created_at, updated_at, structure_id, structure_custom, ai_instructions, phase, auto_tag_enabled;

-- name: ArchiveProject :exec
UPDATE projects SET archived = true, updated_at = now() WHERE id = $1;

-- name: DeleteProject :exec
DELETE FROM projects WHERE id = $1;

-- name: GetAIInstructions :one
SELECT ai_instructions FROM projects WHERE id = $1;

-- name: UpdateAIInstructions :exec
UPDATE projects SET ai_instructions = $2, updated_at = now() WHERE id = $1;

-- name: GetProjectPhase :one
SELECT phase FROM projects WHERE id = $1;

-- name: UpdateProjectPhase :exec
UPDATE projects SET phase = $2, updated_at = now() WHERE id = $1;

-- name: GetProjectStats :one
SELECT
    COUNT(DISTINCT s.id)::INT         AS scene_count,
    COUNT(DISTINCT c.id)::INT         AS chapter_count,
    COALESCE(SUM(s.word_count), 0)::INT AS total_word_count,
    GREATEST(
        p.updated_at,
        MAX(c.updated_at),
        MAX(s.updated_at)
    )                                  AS last_updated_at
FROM projects p
LEFT JOIN acts a    ON a.project_id = p.id
LEFT JOIN chapters c ON c.act_id   = a.id
LEFT JOIN scenes s   ON s.chapter_id = c.id
WHERE p.id = $1
GROUP BY p.id, p.updated_at;
