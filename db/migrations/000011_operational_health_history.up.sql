ALTER TABLE relays
  ADD COLUMN IF NOT EXISTS sync_ok BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS handshake_peer_count INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS last_handshake_at TIMESTAMPTZ;

ALTER TABLE subnet_routes
  ADD COLUMN IF NOT EXISTS gateway_history JSONB NOT NULL DEFAULT '[]'::jsonb;

CREATE TABLE IF NOT EXISTS subnet_route_conflicts (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
  cidr CIDR NOT NULL,
  details TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  resolved_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS subnet_route_conflicts_user_created_idx
  ON subnet_route_conflicts(user_id, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS subnet_route_conflicts_open_unique_idx
  ON subnet_route_conflicts(device_id, cidr, details)
  WHERE resolved_at IS NULL;

CREATE OR REPLACE FUNCTION sdwan_probe_reachability(probes JSONB)
RETURNS JSONB
LANGUAGE SQL
IMMUTABLE
AS $$
  SELECT COALESCE(jsonb_agg(jsonb_build_object('target', item->>'target', 'reachable', item->'reachable') ORDER BY item->>'target'), '[]'::jsonb)
  FROM jsonb_array_elements(COALESCE(probes, '[]'::jsonb)) AS item;
$$;

CREATE OR REPLACE FUNCTION sdwan_record_subnet_route_history()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
  event_type TEXT;
  history JSONB;
BEGIN
  IF TG_OP = 'INSERT' THEN
    event_type := 'route_created';
  ELSE
    IF OLD.status IS DISTINCT FROM NEW.status
       OR OLD.approved IS DISTINCT FROM NEW.approved
       OR OLD.advertised IS DISTINCT FROM NEW.advertised THEN
      event_type := 'route_state_changed';
    ELSIF OLD.gateway_enabled IS DISTINCT FROM NEW.gateway_enabled
       OR OLD.gateway_error IS DISTINCT FROM NEW.gateway_error
       OR OLD.gateway_out_interface IS DISTINCT FROM NEW.gateway_out_interface
       OR OLD.gateway_route_interface IS DISTINCT FROM NEW.gateway_route_interface
       OR OLD.gateway_lan_target IS DISTINCT FROM NEW.gateway_lan_target
       OR OLD.gateway_lan_reachable IS DISTINCT FROM NEW.gateway_lan_reachable
       OR sdwan_probe_reachability(OLD.gateway_service_status::jsonb) IS DISTINCT FROM sdwan_probe_reachability(NEW.gateway_service_status::jsonb) THEN
      event_type := 'gateway_health_changed';
    ELSE
      RETURN NEW;
    END IF;
  END IF;

  history := COALESCE(NEW.gateway_history, '[]'::jsonb) || jsonb_build_array(jsonb_build_object(
    'at', now(),
    'event', event_type,
    'status', NEW.status,
    'approved', NEW.approved,
    'advertised', NEW.advertised,
    'gateway_enabled', NEW.gateway_enabled,
    'gateway_error', NEW.gateway_error,
    'out_interface', NEW.gateway_out_interface,
    'route_interface', NEW.gateway_route_interface,
    'lan_target', NEW.gateway_lan_target,
    'lan_reachable', NEW.gateway_lan_reachable,
    'service_probes', COALESCE(NEW.gateway_service_status::jsonb, '[]'::jsonb)
  ));
  IF jsonb_array_length(history) > 30 THEN
    history := history - 0;
  END IF;
  NEW.gateway_history := history;
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS subnet_route_history_trigger ON subnet_routes;
CREATE TRIGGER subnet_route_history_trigger
BEFORE INSERT OR UPDATE ON subnet_routes
FOR EACH ROW EXECUTE FUNCTION sdwan_record_subnet_route_history();
