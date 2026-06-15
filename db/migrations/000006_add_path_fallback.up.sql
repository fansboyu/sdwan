ALTER TABLE users
  ADD COLUMN IF NOT EXISTS path_mode TEXT NOT NULL DEFAULT 'direct'
  CHECK (path_mode IN ('direct', 'auto', 'relay'));

UPDATE users
SET path_mode = CASE WHEN relay_mode THEN 'relay' ELSE 'direct' END;

CREATE TABLE IF NOT EXISTS peer_paths (
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  client_device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
  main_site_device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
  current_path TEXT NOT NULL DEFAULT 'direct' CHECK (current_path IN ('direct', 'relay')),
  desired_path TEXT NOT NULL DEFAULT 'direct' CHECK (desired_path IN ('direct', 'relay')),
  state TEXT NOT NULL DEFAULT 'direct',
  generation BIGINT NOT NULL DEFAULT 1,
  switched_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, client_device_id)
);

CREATE TABLE IF NOT EXISTS device_peer_stats (
  device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
  peer_public_key TEXT NOT NULL,
  latest_handshake_at TIMESTAMPTZ,
  rx_bytes BIGINT NOT NULL DEFAULT 0,
  tx_bytes BIGINT NOT NULL DEFAULT 0,
  last_rx_at TIMESTAMPTZ,
  reported_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (device_id, peer_public_key)
);

CREATE TABLE IF NOT EXISTS device_path_applied (
  device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
  client_device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
  generation BIGINT NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (device_id, client_device_id)
);

CREATE INDEX IF NOT EXISTS device_peer_stats_reported_at_idx
ON device_peer_stats(reported_at);
