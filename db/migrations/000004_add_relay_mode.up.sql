ALTER TABLE users
  ADD COLUMN IF NOT EXISTS relay_mode BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE relays
  ADD COLUMN IF NOT EXISTS relay_token_hash TEXT,
  ADD COLUMN IF NOT EXISTS virtual_ip INET NOT NULL DEFAULT '100.254.253.1';

CREATE UNIQUE INDEX IF NOT EXISTS relays_token_hash_key
ON relays(relay_token_hash)
WHERE relay_token_hash IS NOT NULL;
