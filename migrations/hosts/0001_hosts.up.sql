CREATE SCHEMA IF NOT EXISTS hosts;
CREATE TABLE hosts.hosts(id uuid PRIMARY KEY,location_id uuid NOT NULL,name text NOT NULL,hostname text,address text NOT NULL,operating_system text NOT NULL CHECK(operating_system IN ('linux','windows','other')),description text NOT NULL DEFAULT '',enabled boolean NOT NULL DEFAULT true,status text NOT NULL DEFAULT 'unknown' CHECK(status IN ('unknown','online','offline')),created_by uuid NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now(),deleted_at timestamptz);
CREATE UNIQUE INDEX hosts_name_location_uidx ON hosts.hosts(location_id,lower(name)) WHERE deleted_at IS NULL;
CREATE TABLE hosts.tags(host_id uuid NOT NULL REFERENCES hosts.hosts(id) ON DELETE CASCADE,tag text NOT NULL,PRIMARY KEY(host_id,tag));
CREATE TABLE hosts.inventory_snapshots(id uuid PRIMARY KEY,host_id uuid NOT NULL REFERENCES hosts.hosts(id) ON DELETE CASCADE,collected_at timestamptz NOT NULL,inventory jsonb NOT NULL);
