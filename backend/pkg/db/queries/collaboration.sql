-- name: CreateInvite :one
INSERT INTO project_invites (project_id, invited_by, email, role, token, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetInviteByToken :one
SELECT * FROM project_invites WHERE token = $1;

-- name: AcceptInvite :exec
UPDATE project_invites SET accepted_at = now() WHERE token = $1;

-- name: ListPendingInvites :many
SELECT * FROM project_invites
WHERE project_id = $1
  AND accepted_at IS NULL
  AND expires_at > now()
ORDER BY created_at DESC;

-- name: DeleteInvite :exec
DELETE FROM project_invites WHERE id = $1 AND project_id = $2;

-- name: CreateCollaborator :one
INSERT INTO project_collaborators (project_id, user_id, role, branch_name, clone_path, invited_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetCollaborator :one
SELECT * FROM project_collaborators WHERE project_id = $1 AND user_id = $2;

-- name: ListCollaborators :many
SELECT * FROM project_collaborators WHERE project_id = $1 ORDER BY joined_at;

-- name: RemoveCollaborator :exec
DELETE FROM project_collaborators WHERE project_id = $1 AND user_id = $2;

-- name: IsCollaborator :one
SELECT EXISTS (
  SELECT 1 FROM project_collaborators WHERE project_id = $1 AND user_id = $2
) AS exists;
