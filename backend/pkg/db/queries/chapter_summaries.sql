-- name: UpsertChapterSummary :exec
INSERT INTO chapter_summaries (chapter_id, branch_name, ai_summary, stale, updated_at)
VALUES ($1, $2, $3, FALSE, NOW())
ON CONFLICT (chapter_id, branch_name)
DO UPDATE SET ai_summary = EXCLUDED.ai_summary,
              stale      = FALSE,
              updated_at = NOW();

-- name: GetChapterSummary :one
SELECT chapter_id, branch_name, ai_summary, stale, updated_at
FROM chapter_summaries
WHERE chapter_id = $1 AND branch_name = $2;

-- name: MarkChapterSummaryStale :exec
INSERT INTO chapter_summaries (chapter_id, branch_name, stale, updated_at)
VALUES ($1, $2, TRUE, NOW())
ON CONFLICT (chapter_id, branch_name)
DO UPDATE SET stale      = TRUE,
              updated_at = NOW();

-- name: ListChapterSummariesByProject :many
-- Returns all summary rows for chapters belonging to a project, ordered by
-- chapter sort_order. Used by BuildContext to assemble the AI context window.
SELECT cs.chapter_id,
       cs.branch_name,
       cs.ai_summary,
       cs.stale,
       cs.updated_at,
       c.title      AS chapter_title,
       c.sort_order AS chapter_sort_order
FROM chapter_summaries cs
JOIN chapters c ON c.id = cs.chapter_id
WHERE c.project_id  = $1
  AND cs.branch_name = $2
ORDER BY c.sort_order ASC;

-- name: DeleteChapterSummariesByBranch :exec
-- Called by Canonize to remove the merged branch's summary rows.
DELETE FROM chapter_summaries
WHERE branch_name = $1
  AND chapter_id IN (
      SELECT c.id FROM chapters c WHERE c.project_id = $2
  );

-- name: UpsertProjectActiveBranch :exec
INSERT INTO project_active_branch (project_id, user_id, branch_name, updated_at)
VALUES ($1, $2, $3, NOW())
ON CONFLICT (project_id, user_id)
DO UPDATE SET branch_name = EXCLUDED.branch_name,
              updated_at  = NOW();

-- name: GetProjectActiveBranch :one
SELECT branch_name
FROM project_active_branch
WHERE project_id = $1 AND user_id = $2;

-- name: DeleteProjectActiveBranchByBranch :exec
-- Called by Canonize to clear all user pointers to the merged branch.
DELETE FROM project_active_branch
WHERE project_id = $1 AND branch_name = $2;
