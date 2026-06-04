ALTER TABLE subscriptions
  DROP COLUMN IF EXISTS free_months,
  DROP COLUMN IF EXISTS source;
