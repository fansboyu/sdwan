DROP INDEX IF EXISTS devices_one_active_main_site;

ALTER TABLE devices
  DROP COLUMN IF EXISTS site_role;
