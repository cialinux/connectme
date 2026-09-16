CREATE TABLE connections.ssh_host_keys (
 connection_id uuid NOT NULL REFERENCES connections.connections(id) ON DELETE CASCADE,
 address text NOT NULL,
 port integer NOT NULL CHECK(port BETWEEN 1 AND 65535),
 public_key text NOT NULL,
 updated_by uuid NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(connection_id,address,port)
);
CREATE TABLE connections.ssh_host_key_events (
 id bigserial PRIMARY KEY,
 connection_id uuid NOT NULL REFERENCES connections.connections(id) ON DELETE CASCADE,
 address text NOT NULL,
 port integer NOT NULL,
 previous_key text NOT NULL,
 public_key text NOT NULL,
 actor_id uuid NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
