CREATE TABLE users (
    id            INTEGER PRIMARY KEY,
    username      TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    password_hash TEXT    NOT NULL,
    is_admin      INTEGER NOT NULL DEFAULT 0,
    created_at    INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE sessions (
    token_hash TEXT    PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at INTEGER NOT NULL
);

CREATE TABLE profiles (
    id         INTEGER PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name       TEXT    NOT NULL,
    avatar     TEXT    NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE my_list (
    profile_id INTEGER NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    media_type TEXT    NOT NULL,
    tmdb_id    INTEGER NOT NULL,
    title      TEXT    NOT NULL,
    poster     TEXT    NOT NULL,
    added_at   INTEGER NOT NULL DEFAULT (unixepoch()),
    PRIMARY KEY (profile_id, media_type, tmdb_id)
);

-- One row per movie, or per episode of a show (season/episode are 0 for movies).
CREATE TABLE progress (
    profile_id INTEGER NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    media_type TEXT    NOT NULL,
    tmdb_id    INTEGER NOT NULL,
    season     INTEGER NOT NULL DEFAULT 0,
    episode    INTEGER NOT NULL DEFAULT 0,
    title      TEXT    NOT NULL,
    poster     TEXT    NOT NULL,
    backdrop   TEXT    NOT NULL,
    position   REAL    NOT NULL,
    duration   REAL    NOT NULL,
    updated_at INTEGER NOT NULL DEFAULT (unixepoch()),
    PRIMARY KEY (profile_id, media_type, tmdb_id, season, episode)
);

-- url is synced from vStream's sites.json, url_override is set by an admin and wins.
CREATE TABLE sources (
    id           TEXT    PRIMARY KEY,
    enabled      INTEGER NOT NULL DEFAULT 1,
    url          TEXT    NOT NULL DEFAULT '',
    url_override TEXT    NOT NULL DEFAULT ''
);

CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
