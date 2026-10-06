-- Sign-in from another device: the TV shows code, a signed-in user enters it (user_id set), the TV polls with the
-- secret token and gets a session.
CREATE TABLE device_codes (
    code       TEXT   PRIMARY KEY,
    token_hash TEXT   NOT NULL UNIQUE,
    user_id    BIGINT REFERENCES users (id) ON DELETE CASCADE,
    expires_at BIGINT NOT NULL
);
