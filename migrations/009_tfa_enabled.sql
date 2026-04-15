-- Add two-factor authentication flag (disabled by default)
ALTER TABLE users ADD COLUMN IF NOT EXISTS tfa_enabled BOOLEAN NOT NULL DEFAULT FALSE;
