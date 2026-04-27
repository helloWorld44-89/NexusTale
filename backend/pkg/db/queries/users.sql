-- name: CreateUser :one
INSERT INTO users (email, display_name, password_hash, role)
VALUES ($1, $2, $3, $4)
RETURNING id, email, display_name, role, plan, created_at, updated_at;

-- name: GetUserByEmail :one
SELECT id, email, display_name, password_hash, role, plan, created_at, updated_at
FROM users
WHERE email = $1;

-- name: GetUserByID :one
SELECT id, email, display_name, password_hash, role, plan, created_at, updated_at
FROM users
WHERE id = $1;

-- name: UpdateUser :one
UPDATE users
SET display_name = COALESCE(sqlc.narg('display_name'), display_name),
    email = COALESCE(sqlc.narg('email'), email),
    updated_at = now()
WHERE id = $1
RETURNING id, email, display_name, role, plan, created_at, updated_at;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;

-- name: ListProjectGitPaths :many
SELECT git_repo_path FROM projects WHERE owner_id = $1 AND git_repo_path != '';

-- name: ListUserWikiImageKeys :many
SELECT we.image_key
FROM wiki_entities we
JOIN projects p ON p.id = we.project_id
WHERE p.owner_id = $1
  AND we.image_key IS NOT NULL
  AND we.image_key != '';

-- name: ListUserExportMinioKeys :many
SELECT minio_key
FROM export_jobs
WHERE user_id = $1
  AND minio_key IS NOT NULL
  AND minio_key != '';

-- name: ListUserCollaboratorClonePaths :many
SELECT clone_path
FROM project_collaborators
WHERE user_id = $1
  AND clone_path IS NOT NULL
  AND clone_path != '';
