ALTER TABLE credentials.secret_refs ADD COLUMN enabled boolean NOT NULL DEFAULT true;
ALTER TABLE credentials.secret_refs ADD COLUMN username text NOT NULL DEFAULT '';
ALTER TABLE credentials.secret_refs ADD COLUMN domain text NOT NULL DEFAULT '';
