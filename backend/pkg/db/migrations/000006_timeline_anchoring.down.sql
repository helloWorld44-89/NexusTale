DROP INDEX IF EXISTS idx_wiki_timeline_anchor;

ALTER TABLE wiki_timeline_events
    DROP COLUMN IF EXISTS anchor_offset_day,
    DROP COLUMN IF EXISTS anchor_offset_month,
    DROP COLUMN IF EXISTS anchor_offset_year,
    DROP COLUMN IF EXISTS anchor_event_id;
