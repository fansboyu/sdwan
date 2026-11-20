CREATE UNIQUE INDEX IF NOT EXISTS subnet_route_conflicts_open_unique_idx
  ON subnet_route_conflicts(device_id, cidr, details)
  WHERE resolved_at IS NULL;
