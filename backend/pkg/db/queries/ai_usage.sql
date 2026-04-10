-- name: InsertUsage :exec
INSERT INTO ai_usage (user_id, project_id, model, prompt_tokens, completion_tokens, cost_usd)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetProjectUsageSummary :one
SELECT
    COALESCE(SUM(prompt_tokens + completion_tokens), 0)::bigint        AS total_tokens,
    COALESCE(SUM(cost_usd), 0)                                         AS total_cost_usd,
    COALESCE(SUM(CASE WHEN created_at >= date_trunc('month', now())
                      THEN prompt_tokens + completion_tokens END), 0)::bigint  AS monthly_tokens,
    COALESCE(SUM(CASE WHEN created_at >= date_trunc('month', now())
                      THEN cost_usd END), 0)                           AS monthly_cost_usd,
    COUNT(CASE WHEN created_at >= date_trunc('month', now()) THEN 1 END)::bigint AS calls_this_month
FROM ai_usage
WHERE project_id = $1;
