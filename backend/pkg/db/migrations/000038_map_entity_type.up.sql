-- Adds 'map' to wiki_entities.type so maps can be stored as wiki entities
-- (D-Map M1) — gets parent_entity_id hierarchy and wiki_relationships edges
-- for free instead of a standalone table.
ALTER TABLE wiki_entities DROP CONSTRAINT wiki_entities_type_check;
ALTER TABLE wiki_entities ADD CONSTRAINT wiki_entities_type_check
    CHECK (type IN ('character', 'location', 'faction', 'item', 'concept', 'lore', 'map'));
