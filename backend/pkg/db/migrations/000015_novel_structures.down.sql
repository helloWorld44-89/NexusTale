ALTER TABLE projects
    DROP COLUMN IF EXISTS structure_custom,
    DROP COLUMN IF EXISTS structure_id;

DROP TABLE IF EXISTS novel_structures;
