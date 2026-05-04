ALTER TABLE wiki_magic_rules
    ADD COLUMN attributes JSONB NOT NULL DEFAULT '{}';
