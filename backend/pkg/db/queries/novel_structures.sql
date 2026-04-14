-- name: ListNovelStructures :many
SELECT id, name, description, phases, strengths, risks, sort_order
FROM novel_structures
ORDER BY sort_order;

-- name: GetNovelStructure :one
SELECT id, name, description, phases, strengths, risks, sort_order
FROM novel_structures
WHERE id = $1;

-- name: GetProjectStructure :one
SELECT
    p.structure_id,
    ns.name        AS structure_name,
    ns.phases      AS phases,
    p.structure_custom
FROM projects p
LEFT JOIN novel_structures ns ON ns.id = p.structure_id
WHERE p.id = $1;

-- name: UpdateProjectStructure :one
UPDATE projects
SET structure_id     = sqlc.narg('structure_id'),
    structure_custom = sqlc.narg('structure_custom'),
    updated_at       = now()
WHERE id = $1
RETURNING structure_id, structure_custom;
