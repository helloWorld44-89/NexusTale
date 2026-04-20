-- name: CreateMergeRequest :one
INSERT INTO merge_requests (project_id, from_branch, to_branch, title, description, requested_by)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetMergeRequest :one
SELECT mr.id, mr.project_id, mr.from_branch, mr.to_branch, mr.title, mr.description,
       mr.requested_by, u.display_name AS requester_name,
       mr.status, mr.reviewer_note, mr.created_at, mr.resolved_at
FROM merge_requests mr
JOIN users u ON u.id = mr.requested_by
WHERE mr.id = $1;

-- name: ListMergeRequests :many
SELECT mr.id, mr.project_id, mr.from_branch, mr.to_branch, mr.title, mr.description,
       mr.requested_by, u.display_name AS requester_name,
       mr.status, mr.reviewer_note, mr.created_at, mr.resolved_at
FROM merge_requests mr
JOIN users u ON u.id = mr.requested_by
WHERE mr.project_id = $1
ORDER BY mr.created_at DESC;

-- name: GetOpenMRByBranch :one
SELECT id FROM merge_requests
WHERE project_id = $1 AND from_branch = $2 AND status = 'open'
LIMIT 1;

-- name: UpdateMergeRequestStatus :one
UPDATE merge_requests
SET status = $2, reviewer_note = $3, resolved_at = CASE WHEN $2 != 'open' THEN now() ELSE NULL END
WHERE id = $1
RETURNING *;
