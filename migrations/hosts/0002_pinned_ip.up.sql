ALTER TABLE hosts.hosts ADD COLUMN pinned_ip inet;
CREATE INDEX hosts_pinned_ip_idx ON hosts.hosts(location_id,pinned_ip) WHERE deleted_at IS NULL AND enabled;
