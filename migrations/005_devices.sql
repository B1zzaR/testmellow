-- ============================================================
-- Migration 005: device management
-- ============================================================

CREATE TABLE IF NOT EXISTS devices (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_name TEXT        NOT NULL DEFAULT 'Unknown Device',
    last_active TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    is_active   BOOLEAN     NOT NULL DEFAULT TRUE
);

CREATE INDEX IF NOT EXISTS idx_devices_user_id    ON devices(user_id);
CREATE INDEX IF NOT EXISTS idx_devices_last_active ON devices(last_active);
