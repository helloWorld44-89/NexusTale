-- name: CreateNotification :one
INSERT INTO notifications (user_id, project_id, type, payload)
VALUES ($1, $2, $3, $4)
RETURNING id, user_id, project_id, type, payload, read_at, created_at;

-- name: ListNotifications :many
-- Returns all unread notifications + the last 20 read ones, newest first.
SELECT id, user_id, project_id, type, payload, read_at, created_at
FROM notifications
WHERE user_id = $1
  AND (read_at IS NULL OR created_at > now() - INTERVAL '30 days')
ORDER BY
  (read_at IS NULL) DESC,
  created_at DESC
LIMIT 50;

-- name: MarkNotificationRead :execrows
-- Idempotent: re-marking a read notification keeps its original read_at and
-- still counts as a hit, so zero rows means "no such notification for this user".
UPDATE notifications
SET read_at = COALESCE(read_at, now())
WHERE id = $1
  AND user_id = $2;

-- name: MarkAllNotificationsRead :exec
UPDATE notifications
SET read_at = now()
WHERE user_id = $1
  AND read_at IS NULL;

-- name: UnreadCount :one
SELECT COUNT(*)::int AS count
FROM notifications
WHERE user_id = $1
  AND read_at IS NULL;
