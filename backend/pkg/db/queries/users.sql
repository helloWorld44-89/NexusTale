-- name: CreateUser :one
INSERT INTO users (email, display_name, password_hash, role)
VALUES ($1, $2, $3, $4)
RETURNING id, email, display_name, role, created_at, updated_at;

-- name: GetUserByEmail :one
SELECT id, email, display_name, password_hash, role, created_at, updated_at
FROM users
WHERE email = $1;

-- name: GetUserByID :one
SELECT id, email, display_name, password_hash, role, created_at, updated_at
FROM users
WHERE id = $1;

-- name: UpdateUser :one
UPDATE users
SET display_name = COALESCE(sqlc.narg('display_name'), display_name),
    email = COALESCE(sqlc.narg('email'), email),
    updated_at = now()
WHERE id = $1
RETURNING id, email, display_name, role, created_at, updated_at;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;

-- name: ListProjectGitPaths :many
SELECT git_repo_path FROM projects WHERE owner_id = $1 AND git_repo_path != '';
