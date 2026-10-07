-- "Family list": like my_list, but shared by every profile of the account.
CREATE TABLE family_list (
    user_id    BIGINT  NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    media_type TEXT    NOT NULL,
    tmdb_id    INTEGER NOT NULL,
    title      TEXT    NOT NULL,
    poster     TEXT    NOT NULL,
    added_at   BIGINT  NOT NULL DEFAULT extract(epoch FROM now())::bigint,
    PRIMARY KEY (user_id, media_type, tmdb_id)
);
