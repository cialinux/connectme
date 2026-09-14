ALTER TABLE locations.locations ADD COLUMN deleted_at timestamptz;
ALTER TABLE locations.networks ADD COLUMN deleted_at timestamptz;
