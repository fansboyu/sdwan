ALTER TABLE relays
  ADD COLUMN IF NOT EXISTS interface_up BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS synced_peer_count INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS version TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS last_error TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS last_sync_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS device_path_quality (
  device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
  peer_device_id TEXT NOT NULL,
  path_type TEXT NOT NULL CHECK (path_type IN ('direct', 'relay')),
  current_path BOOLEAN NOT NULL DEFAULT false,
  endpoint TEXT NOT NULL DEFAULT '',
  latest_handshake_at TIMESTAMPTZ,
  rx_bytes BIGINT NOT NULL DEFAULT 0,
  tx_bytes BIGINT NOT NULL DEFAULT 0,
  last_rx_at TIMESTAMPTZ,
  fail_count INTEGER NOT NULL DEFAULT 0,
  last_ok_at TIMESTAMPTZ,
  last_failed_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (device_id, peer_device_id, path_type)
);

CREATE INDEX IF NOT EXISTS device_path_quality_updated_at_idx
ON device_path_quality(updated_at);
