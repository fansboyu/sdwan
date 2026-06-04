ALTER TABLE devices
  ADD COLUMN IF NOT EXISTS site_role TEXT NOT NULL DEFAULT 'client';

CREATE UNIQUE INDEX IF NOT EXISTS devices_one_active_main_site
ON devices(user_id)
WHERE site_role = 'main_site' AND status = 'active';
