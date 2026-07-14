-- migration 036: prose_fingerprint
-- Stores per-project prose statistics extracted from the writer's own scenes.
-- Used by C9-P5 to inject an "## Author's prose style" block into Beat/Continue
-- prompts so the AI matches the writer's rhythm without manual style configuration.
ALTER TABLE projects ADD COLUMN prose_fingerprint JSONB;
