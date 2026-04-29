-- ============================
-- Waitlist Signups (alpha pre-launch)
-- ============================

-- name: CreateWaitlistSignup :one
-- Upsert by email so duplicate submissions are idempotent.
INSERT INTO waitlist_signups (email, what_they_write)
VALUES ($1, $2)
ON CONFLICT (email) DO UPDATE
    SET what_they_write = EXCLUDED.what_they_write
RETURNING id, email, what_they_write, created_at;

-- name: ListWaitlistSignups :many
SELECT id, email, what_they_write, created_at
FROM   waitlist_signups
ORDER  BY created_at DESC;
