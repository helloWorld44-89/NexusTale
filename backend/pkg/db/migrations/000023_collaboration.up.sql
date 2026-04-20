-- Extend the existing project_collaborators stub (migration 003) with the
-- columns needed for git-backed collaboration. The stub had only project_id,
-- user_id, role (user_role enum), and created_at.

-- Replace the user_role enum column with a TEXT column that uses the
-- collaboration-specific role values.
ALTER TABLE project_collaborators DROP COLUMN role;
ALTER TABLE project_collaborators ADD COLUMN role TEXT NOT NULL DEFAULT 'reviewer'
  CHECK (role IN ('coauthor', 'editor', 'reviewer'));

-- Git clone path and branch name for the collaborator's working tree.
ALTER TABLE project_collaborators ADD COLUMN branch_name TEXT NOT NULL DEFAULT '';
ALTER TABLE project_collaborators ADD COLUMN clone_path  TEXT NOT NULL DEFAULT '';

-- Who sent the invite.
ALTER TABLE project_collaborators ADD COLUMN invited_by UUID REFERENCES users(id);

-- Rename created_at → joined_at for clarity; add index on user_id for
-- the "list all projects for user" query.
ALTER TABLE project_collaborators RENAME COLUMN created_at TO joined_at;
CREATE INDEX ON project_collaborators (user_id);

-- Pending invite tokens.
CREATE TABLE project_invites (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id  UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  invited_by  UUID NOT NULL REFERENCES users(id),
  email       TEXT NOT NULL,
  role        TEXT NOT NULL CHECK (role IN ('coauthor', 'editor', 'reviewer')),
  token       TEXT NOT NULL UNIQUE,
  accepted_at TIMESTAMPTZ,
  expires_at  TIMESTAMPTZ NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ON project_invites (project_id);
CREATE INDEX ON project_invites (token);
