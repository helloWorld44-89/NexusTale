-- NexusTale Alpha Metrics
-- Run on the alpha VM:
--   docker exec nexustale-db psql -U nexustale -d nexustale -f /tmp/alpha-metrics.sql
-- Or interactively:
--   docker exec -it nexustale-db psql -U nexustale -d nexustale

\echo ''
\echo '======================================================'
\echo '  ALPHA GRADUATION CRITERIA'
\echo '======================================================'

-- ── 1. Guide wizard completions ───────────────────────────────────────────────
-- Criterion: ≥10 writers have completed all 5 steps (premise → first_scene)
\echo ''
\echo '--- 1. Guide wizard completions (need ≥10) ---'
SELECT
    u.display_name,
    u.email,
    p.title AS project,
    COUNT(gs.step_key) FILTER (WHERE gs.completed_at IS NOT NULL) AS steps_done,
    COUNT(gs.step_key) AS steps_saved,
    MAX(gs.completed_at) AS last_step_at,
    CASE WHEN COUNT(gs.step_key) FILTER (WHERE gs.completed_at IS NOT NULL) = 5
         THEN '✓ complete' ELSE '…in progress' END AS status
FROM guide_steps gs
JOIN projects p ON p.id = gs.project_id
JOIN users u ON u.id = p.owner_id
GROUP BY u.display_name, u.email, p.title
ORDER BY steps_done DESC, last_step_at DESC;

\echo ''
\echo '--- Guide completions total ---'
SELECT COUNT(*) AS total_complete
FROM (
    SELECT project_id
    FROM guide_steps
    WHERE completed_at IS NOT NULL
    GROUP BY project_id
    HAVING COUNT(*) = 5
) finished;

-- ── 2. Collaborative MRs resolved ────────────────────────────────────────────
-- Criterion: ≥3 collaborative projects with at least one merged/approved MR
\echo ''
\echo '--- 2. Collaborative projects with resolved MRs (need ≥3) ---'
SELECT
    p.title AS project,
    u.display_name AS owner,
    COUNT(*) FILTER (WHERE mr.status IN ('merged', 'approved')) AS resolved_mrs,
    COUNT(*) FILTER (WHERE mr.status = 'open')   AS open_mrs,
    COUNT(*) FILTER (WHERE mr.status = 'rejected') AS rejected_mrs
FROM merge_requests mr
JOIN projects p ON p.id = mr.project_id
JOIN users u ON u.id = p.owner_id
GROUP BY p.title, u.display_name
HAVING COUNT(*) FILTER (WHERE mr.status IN ('merged', 'approved')) > 0
ORDER BY resolved_mrs DESC;

\echo ''
\echo '--- Qualifying collab projects total ---'
SELECT COUNT(DISTINCT project_id) AS projects_with_resolved_mr
FROM merge_requests
WHERE status IN ('merged', 'approved');

-- ── 3. Core user loop: register → write → export ─────────────────────────────
-- Chronicle is a git op (no DB row). Proxy: user has scenes with words + an export job done.
-- Run `git log --oneline` in the repo dir on the VM to verify Chronicle activity separately.
\echo ''
\echo '--- 3. Core user loop completion (need ≥5 non-dev users) ---'
SELECT
    u.display_name,
    u.email,
    u.created_at::date AS registered,
    COUNT(DISTINCT p.id) AS projects,
    COALESCE(SUM(s.word_count), 0) AS total_words,
    COUNT(DISTINCT ej.id) FILTER (WHERE ej.status = 'done') AS exports_done,
    CASE
        WHEN COALESCE(SUM(s.word_count), 0) > 0
         AND COUNT(DISTINCT ej.id) FILTER (WHERE ej.status = 'done') > 0
        THEN '✓ wrote + exported'
        WHEN COALESCE(SUM(s.word_count), 0) > 0
        THEN '…wrote, no export yet'
        ELSE '…no content yet'
    END AS loop_status
FROM users u
LEFT JOIN projects p ON p.owner_id = u.id
LEFT JOIN chapters ch ON ch.project_id = p.id
LEFT JOIN scenes s ON s.chapter_id = ch.id
LEFT JOIN export_jobs ej ON ej.user_id = u.id
WHERE u.role != 'admin'
GROUP BY u.id, u.display_name, u.email, u.created_at
ORDER BY total_words DESC;


\echo ''
\echo '======================================================'
\echo '  ENGAGEMENT OVERVIEW'
\echo '======================================================'

-- ── Active users ──────────────────────────────────────────────────────────────
\echo ''
\echo '--- Active users (AI calls as proxy for engagement) ---'
SELECT
    '7d'  AS window,
    COUNT(DISTINCT user_id) AS active_users
FROM ai_usage WHERE created_at > now() - interval '7 days'
UNION ALL
SELECT '30d', COUNT(DISTINCT user_id)
FROM ai_usage WHERE created_at > now() - interval '30 days'
UNION ALL
SELECT 'all time', COUNT(DISTINCT user_id)
FROM ai_usage;

-- ── Registrations over time ───────────────────────────────────────────────────
\echo ''
\echo '--- Registrations by day ---'
SELECT
    created_at::date AS day,
    COUNT(*) AS new_users
FROM users
WHERE role != 'admin'
GROUP BY day
ORDER BY day DESC
LIMIT 30;

-- ── Writing activity ──────────────────────────────────────────────────────────
\echo ''
\echo '--- Writing activity per user ---'
SELECT
    u.display_name,
    u.email,
    COUNT(DISTINCT p.id)  AS projects,
    COUNT(DISTINCT ch.id) AS chapters,
    COUNT(DISTINCT s.id)  AS scenes,
    COALESCE(SUM(s.word_count), 0) AS total_words,
    MAX(s.updated_at)::date AS last_wrote
FROM users u
LEFT JOIN projects p  ON p.owner_id   = u.id
LEFT JOIN chapters ch ON ch.project_id = p.id
LEFT JOIN scenes s    ON s.chapter_id  = ch.id
WHERE u.role != 'admin'
GROUP BY u.id, u.display_name, u.email
ORDER BY total_words DESC;

-- ── Total content in the system ───────────────────────────────────────────────
\echo ''
\echo '--- Platform totals ---'
SELECT
    (SELECT COUNT(*) FROM users WHERE role != 'admin') AS total_users,
    (SELECT COUNT(*) FROM projects)                    AS total_projects,
    (SELECT COUNT(*) FROM chapters)                    AS total_chapters,
    (SELECT COUNT(*) FROM scenes)                      AS total_scenes,
    (SELECT COALESCE(SUM(word_count), 0) FROM scenes)  AS total_words_written;


\echo ''
\echo '======================================================'
\echo '  AI USAGE'
\echo '======================================================'

\echo ''
\echo '--- AI calls by mode ---'
SELECT
    mode,
    COUNT(*)                        AS calls,
    SUM(prompt_tokens)              AS prompt_tokens,
    SUM(completion_tokens)          AS completion_tokens,
    ROUND(SUM(cost_usd)::numeric, 4) AS total_cost_usd
FROM ai_usage
GROUP BY mode
ORDER BY calls DESC;

\echo ''
\echo '--- AI calls by model ---'
SELECT
    model,
    COUNT(*)                         AS calls,
    ROUND(SUM(cost_usd)::numeric, 4) AS cost_usd
FROM ai_usage
GROUP BY model
ORDER BY calls DESC;

\echo ''
\echo '--- AI usage per user (all time) ---'
SELECT
    u.display_name,
    COUNT(au.id)                     AS total_calls,
    SUM(au.prompt_tokens)            AS prompt_tokens,
    SUM(au.completion_tokens)        AS completion_tokens,
    ROUND(SUM(au.cost_usd)::numeric, 4) AS cost_usd,
    MAX(au.created_at)::date         AS last_call
FROM ai_usage au
JOIN users u ON u.id = au.user_id
GROUP BY u.id, u.display_name
ORDER BY total_calls DESC;

\echo ''
\echo '--- Workshop sessions per user ---'
SELECT
    u.display_name,
    COUNT(ws.id) AS sessions,
    MAX(ws.updated_at)::date AS last_session
FROM workshop_sessions ws
JOIN users u ON u.id = ws.user_id
GROUP BY u.id, u.display_name
ORDER BY sessions DESC;


\echo ''
\echo '======================================================'
\echo '  EXPORTS'
\echo '======================================================'

\echo ''
\echo '--- Export jobs by format + status ---'
SELECT
    format,
    status,
    COUNT(*) AS count
FROM export_jobs
GROUP BY format, status
ORDER BY format, status;

\echo ''
\echo '--- Export activity per user ---'
SELECT
    u.display_name,
    COUNT(*) FILTER (WHERE ej.status = 'done')   AS successful,
    COUNT(*) FILTER (WHERE ej.status = 'failed')  AS failed,
    COUNT(*) FILTER (WHERE ej.status = 'pending') AS pending,
    MAX(ej.created_at)::date AS last_export
FROM export_jobs ej
JOIN users u ON u.id = ej.user_id
GROUP BY u.id, u.display_name
ORDER BY successful DESC;


\echo ''
\echo '======================================================'
\echo '  COLLABORATION'
\echo '======================================================'

\echo ''
\echo '--- Collaborator count per project ---'
SELECT
    p.title,
    u.display_name AS owner,
    COUNT(pc.user_id) AS collaborators
FROM projects p
JOIN users u ON u.id = p.owner_id
LEFT JOIN project_collaborators pc ON pc.project_id = p.id
GROUP BY p.id, p.title, u.display_name
HAVING COUNT(pc.user_id) > 0
ORDER BY collaborators DESC;

\echo ''
\echo '--- All merge requests ---'
SELECT
    p.title AS project,
    mr.from_branch,
    mr.status,
    u.display_name AS opened_by,
    mr.created_at::date AS opened,
    mr.resolved_at::date AS resolved
FROM merge_requests mr
JOIN projects p ON p.id = mr.project_id
JOIN users u ON u.id = mr.requested_by
ORDER BY mr.created_at DESC;


\echo ''
\echo '======================================================'
\echo '  WAITLIST'
\echo '======================================================'

\echo ''
\echo '--- Waitlist signups by day ---'
SELECT
    created_at::date AS day,
    COUNT(*) AS signups
FROM waitlist_signups
GROUP BY day
ORDER BY day DESC
LIMIT 30;

\echo ''
\echo '--- Waitlist total ---'
SELECT COUNT(*) AS total_waitlist FROM waitlist_signups;
