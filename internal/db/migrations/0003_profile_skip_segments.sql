-- "Skip intro" / "Skip credits" buttons, on by default; a profile can turn them off.
ALTER TABLE profiles ADD COLUMN skip_segments INTEGER NOT NULL DEFAULT 1;
