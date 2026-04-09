-- Relative anchoring for timeline events.
-- An event can be "X years/months/days after Event Y" instead of specifying
-- absolute year/month/day values directly. The anchor is resolved recursively
-- at read time; only one level of offset arithmetic is stored per event.
--
-- Constraint: anchor_event_id and absolute year/month/day are mutually exclusive.
-- Enforcement happens in the application layer (service.go validation).
ALTER TABLE wiki_timeline_events
    ADD COLUMN anchor_event_id    UUID REFERENCES wiki_timeline_events(id) ON DELETE SET NULL,
    ADD COLUMN anchor_offset_year  INT,
    ADD COLUMN anchor_offset_month INT,
    ADD COLUMN anchor_offset_day   INT;

CREATE INDEX idx_wiki_timeline_anchor ON wiki_timeline_events (anchor_event_id);
