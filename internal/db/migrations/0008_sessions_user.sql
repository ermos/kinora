-- Logging out every session of an account (password change) looks sessions up by user.
CREATE INDEX sessions_user ON sessions (user_id);
