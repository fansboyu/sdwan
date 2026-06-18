ALTER TABLE subnet_routes
  DROP COLUMN IF EXISTS gateway_lan_reachable,
  DROP COLUMN IF EXISTS gateway_lan_target,
  DROP COLUMN IF EXISTS gateway_route_interface,
  DROP COLUMN IF EXISTS gateway_out_interface,
  DROP COLUMN IF EXISTS gateway_checked_at,
  DROP COLUMN IF EXISTS gateway_error,
  DROP COLUMN IF EXISTS gateway_enabled;
