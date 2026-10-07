-- Thumbs up (1) or down (-1) of a profile on a title. A thumbs down hides the title from its family list.
CREATE TABLE ratings (
    profile_id BIGINT   NOT NULL REFERENCES profiles (id) ON DELETE CASCADE,
    media_type TEXT     NOT NULL,
    tmdb_id    INTEGER  NOT NULL,
    title      TEXT     NOT NULL,
    poster     TEXT     NOT NULL,
    rating     SMALLINT NOT NULL CHECK (rating IN (-1, 1)),
    rated_at   BIGINT   NOT NULL DEFAULT extract(epoch FROM now())::bigint,
    PRIMARY KEY (profile_id, media_type, tmdb_id)
);
