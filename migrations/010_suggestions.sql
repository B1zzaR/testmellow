-- 010_suggestions.sql
-- Anonymous suggestion box: stores user suggestions without linking to any user identity.

CREATE TYPE suggestion_status AS ENUM ('new', 'read', 'archived');

CREATE TABLE IF NOT EXISTS suggestions (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    body       TEXT NOT NULL CHECK (char_length(body) BETWEEN 1 AND 3000),
    status     suggestion_status NOT NULL DEFAULT 'new',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS suggestions_status_idx ON suggestions (status);
CREATE INDEX IF NOT EXISTS suggestions_created_at_idx ON suggestions (created_at DESC);
