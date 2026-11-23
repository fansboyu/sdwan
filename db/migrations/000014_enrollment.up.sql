CREATE TABLE device_enrollments (
 id TEXT PRIMARY KEY,
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 token_hash TEXT NOT NULL UNIQUE,
 expires_at TIMESTAMPTZ NOT NULL,
 used_at TIMESTAMPTZ,
 revoked_at TIMESTAMPTZ
);
CREATE INDEX device_enrollments_user_idx ON device_enrollments(user_id, expires_at DESC);
