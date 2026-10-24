-- Rows that would violate the narrower constraint have to go first.
DELETE FROM export_jobs WHERE format = 'docx';
ALTER TABLE export_jobs DROP CONSTRAINT IF EXISTS export_jobs_format_check;
ALTER TABLE export_jobs ADD CONSTRAINT export_jobs_format_check
    CHECK (format IN ('markdown', 'epub'));
