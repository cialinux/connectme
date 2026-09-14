CREATE SCHEMA IF NOT EXISTS audit;
CREATE TABLE audit.events(id uuid PRIMARY KEY, occurred_at timestamptz NOT NULL, actor_id uuid, action text NOT NULL, outcome text NOT NULL, resource_type text, resource_id uuid, correlation_id text, source_ip inet, metadata jsonb NOT NULL DEFAULT '{}', previous_hash bytea, event_hash bytea NOT NULL, key_version integer NOT NULL DEFAULT 1);
CREATE INDEX audit_events_time_idx ON audit.events(occurred_at DESC);
CREATE INDEX audit_events_actor_idx ON audit.events(actor_id,occurred_at DESC);
CREATE TABLE audit.chain_heads(shard smallint PRIMARY KEY CHECK(shard=0), event_id uuid, event_hash bytea NOT NULL);
INSERT INTO audit.chain_heads(shard,event_hash) VALUES(0,'\\x') ON CONFLICT DO NOTHING;
