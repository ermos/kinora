-- Source URLs only come from vStream's sites.json now: no manual override, no per-source switch.
ALTER TABLE sources DROP COLUMN enabled;
ALTER TABLE sources DROP COLUMN url_override;
