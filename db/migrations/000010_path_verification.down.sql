ALTER TABLE device_path_applied
  DROP COLUMN IF EXISTS verified_at,
  DROP COLUMN IF EXISTS verification_error,
  DROP COLUMN IF EXISTS verification_status,
  DROP COLUMN IF EXISTS effective_path;

ALTER TABLE peer_paths
  DROP COLUMN IF EXISTS verified_at,
  DROP COLUMN IF EXISTS verification_error,
  DROP COLUMN IF EXISTS verification_state,
  DROP COLUMN IF EXISTS verification_updated_at,
  DROP COLUMN IF EXISTS verified_generation,
  DROP COLUMN IF EXISTS verified_path;
