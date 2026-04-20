DROP TABLE IF EXISTS project_invites;

ALTER TABLE project_collaborators RENAME COLUMN joined_at TO created_at;
ALTER TABLE project_collaborators DROP COLUMN IF EXISTS invited_by;
ALTER TABLE project_collaborators DROP COLUMN IF EXISTS clone_path;
ALTER TABLE project_collaborators DROP COLUMN IF EXISTS branch_name;
ALTER TABLE project_collaborators DROP COLUMN IF EXISTS role;
ALTER TABLE project_collaborators ADD COLUMN role user_role NOT NULL DEFAULT 'viewer';
