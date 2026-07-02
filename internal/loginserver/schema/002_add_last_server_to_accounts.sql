-- +migrate Up
-- Persist the last game server an account successfully logged into,
-- so the client can pre-select it on the server selection screen.
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS last_server SMALLINT NOT NULL DEFAULT 0;

-- +migrate Down
ALTER TABLE accounts DROP COLUMN IF EXISTS last_server;
