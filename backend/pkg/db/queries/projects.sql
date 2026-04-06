-- name: CreateProject :one
INSERT INTO projects (owner_id, title, description, genres, git_repo_path)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, owner_id, title, description, genres, git_repo_path, archived, created_at, updated_at;

-- name: GetProject :one
SELECT id, owner_id, title, description, genres, git_repo_path, archived, created_at, updated_at
FROM projects
WHERE id = $1;

-- name: ListProjectsByOwner :many
SELECT id, owner_id, title, description, genres, git_repo_path, archived, created_at, updated_at
FROM projects
WHERE owner_id = $1 AND archived = false
ORDER BY updated_at DESC;

-- name: UpdateProject :one
UPDATE projects
SET title = COALESCE(sqlc.narg('title'), title),
    description = COALESCE(sqlc.narg('description'), description),
    updated_at = now()
WHERE id = $1
RETURNING id, owner_id, title, description, genres, git_repo_path, archived, created_at, updated_at;

-- name: ArchiveProject :exec
UPDATE projects SET archived = true, updated_at = now() WHERE id = $1;

-- name: DeleteProject :exec
DELETE FROM projects WHERE id = $1;
