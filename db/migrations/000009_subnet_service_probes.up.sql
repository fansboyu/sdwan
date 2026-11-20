ALTER TABLE subnet_routes
  ADD COLUMN IF NOT EXISTS gateway_service_status TEXT NOT NULL DEFAULT '[]';
