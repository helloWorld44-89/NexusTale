-- Migration 013 limited export_jobs.format to ('markdown', 'epub'), but DOCX
-- export has been shipping since then: on a fresh database every DOCX job
-- failed at INSERT with a check-constraint violation.
ALTER TABLE export_jobs DROP CONSTRAINT IF EXISTS export_jobs_format_check;
ALTER TABLE export_jobs ADD CONSTRAINT export_jobs_format_check
    CHECK (format IN ('markdown', 'epub', 'docx'));
