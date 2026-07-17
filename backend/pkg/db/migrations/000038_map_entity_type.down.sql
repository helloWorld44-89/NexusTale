ALTER TABLE wiki_entities DROP CONSTRAINT wiki_entities_type_check;
ALTER TABLE wiki_entities ADD CONSTRAINT wiki_entities_type_check
    CHECK (type IN ('character', 'location', 'faction', 'item', 'concept', 'lore'));
