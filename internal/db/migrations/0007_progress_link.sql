-- Link playing when the position was saved, so resuming starts on the same site, hoster and language.
ALTER TABLE progress ADD COLUMN source TEXT NOT NULL DEFAULT '';
ALTER TABLE progress ADD COLUMN hoster TEXT NOT NULL DEFAULT '';
ALTER TABLE progress ADD COLUMN lang   TEXT NOT NULL DEFAULT '';
