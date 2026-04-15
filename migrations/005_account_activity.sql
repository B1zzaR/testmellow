-- Account activity log (logins, password changes, Telegram link/unlink)
CREATE TABLE IF NOT EXISTS account_activity (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_type  VARCHAR(64) NOT NULL,
    ip          INET,
    user_agent  TEXT,
    details     JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_account_activity_user_created ON account_activity(user_id, created_at DESC);

