-- Airing state of every show a profile watched, from TMDB, refreshed in the background so the home page never waits
-- on it. episodes[n + 1] is the episode count of season n (arrays are 1-based, season 0 holds specials).
CREATE TABLE shows (
    tmdb_id       INTEGER   PRIMARY KEY,
    status        TEXT      NOT NULL,
    last_season   INTEGER   NOT NULL,
    last_episode  INTEGER   NOT NULL,
    last_air_date DATE,
    next_air_date DATE,
    episodes      INTEGER[] NOT NULL,
    checked_at    BIGINT    NOT NULL
);
