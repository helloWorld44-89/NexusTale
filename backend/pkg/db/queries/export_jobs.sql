-- name: InsertExportJob :one
INSERT INTO export_jobs (project_id, user_id, format)
VALUES ($1, $2, $3)
RETURNING id, project_id, user_id, format, status, minio_key, error_msg, expires_at, created_at, updated_at;

-- name: GetExportJob :one
SELECT id, project_id, user_id, format, status, minio_key, error_msg, expires_at, created_at, updated_at
FROM export_jobs
WHERE id = $1;

-- name: UpdateExportJobProcessing :exec
UPDATE export_jobs
SET status     = 'processing',
    updated_at = NOW()
WHERE id = $1;

-- name: UpdateExportJobDone :exec
UPDATE export_jobs
SET status     = 'done',
    minio_key  = $2,
    expires_at = $3,
    updated_at = NOW()
WHERE id = $1;

-- name: UpdateExportJobFailed :exec
UPDATE export_jobs
SET status    = 'failed',
    error_msg = $2,
    updated_at = NOW()
WHERE id = $1;

-- name: ListExportJobsByProject :many
SELECT id, project_id, user_id, format, status, minio_key, error_msg, expires_at, created_at, updated_at
FROM export_jobs
WHERE project_id = $1
ORDER BY created_at DESC
LIMIT 20;
