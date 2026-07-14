-- Admin queries — all behind RequireRole(RoleAdmin) in the handler.
-- No per-user scoping: these span the full dataset.

-- name: AdminListUsers :many
-- Paginated user list with per-user project count.
SELECT
    u.id,
    u.email,
    u.display_name,
    u.role,
    u.plan,
    u.created_at,
    COUNT(p.id)::INT AS project_count
FROM users u
LEFT JOIN projects p ON p.owner_id = u.id
GROUP BY u.id
ORDER BY u.created_at DESC
LIMIT  $1
OFFSET $2;

-- name: AdminGetStats :one
SELECT
    (SELECT COUNT(*)::INT FROM users)                   AS total_users,
    (SELECT COUNT(*)::INT FROM projects)                AS total_projects,
    (SELECT COUNT(*)::INT FROM scenes)                  AS total_scenes,
    (SELECT COUNT(*)::INT FROM ai_usage)                AS total_ai_calls,
    COALESCE(SUM(au.prompt_tokens + au.completion_tokens), 0)::BIGINT AS total_tokens,
    COALESCE(SUM(au.cost_usd), 0)                       AS total_cost_usd
FROM ai_usage au;

-- name: AdminListAIUsage :many
-- Per-user AI usage totals for the last 30 days, ordered by token spend.
SELECT
    u.id          AS user_id,
    u.email,
    u.display_name,
    COUNT(*)::INT AS call_count,
    COALESCE(SUM(au.prompt_tokens + au.completion_tokens), 0)::BIGINT AS total_tokens,
    COALESCE(SUM(au.cost_usd), 0) AS total_cost_usd
FROM ai_usage au
JOIN users u ON u.id = au.user_id
WHERE au.created_at >= now() - INTERVAL '30 days'
GROUP BY u.id
ORDER BY total_tokens DESC
LIMIT 50;

-- name: AdminSetUserRole :exec
UPDATE users SET role = $2, updated_at = now() WHERE id = $1;

-- name: AdminSetUserPlan :exec
UPDATE users SET plan = $2, updated_at = now() WHERE id = $1;
