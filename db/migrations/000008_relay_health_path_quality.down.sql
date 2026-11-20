DROP TABLE IF EXISTS device_path_quality;

ALTER TABLE relays
  DROP COLUMN IF EXISTS last_sync_at,
  DROP COLUMN IF EXISTS last_error,
  DROP COLUMN IF EXISTS version,
  DROP COLUMN IF EXISTS synced_peer_count,
  DROP COLUMN IF EXISTS interface_up;
