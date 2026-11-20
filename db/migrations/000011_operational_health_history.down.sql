DROP TRIGGER IF EXISTS subnet_route_history_trigger ON subnet_routes;
DROP FUNCTION IF EXISTS sdwan_record_subnet_route_history();
DROP FUNCTION IF EXISTS sdwan_probe_reachability(JSONB);
DROP TABLE IF EXISTS subnet_route_conflicts;
ALTER TABLE subnet_routes DROP COLUMN IF EXISTS gateway_history;
ALTER TABLE relays
  DROP COLUMN IF EXISTS last_handshake_at,
  DROP COLUMN IF EXISTS handshake_peer_count,
  DROP COLUMN IF EXISTS sync_ok;
