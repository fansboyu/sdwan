DROP INDEX IF EXISTS relays_token_hash_key;

ALTER TABLE relays
  DROP COLUMN IF EXISTS virtual_ip,
  DROP COLUMN IF EXISTS relay_token_hash;

ALTER TABLE users
  DROP COLUMN IF EXISTS relay_mode;
