-- ============================================================
-- Platform Settings
-- ============================================================

CREATE TABLE IF NOT EXISTS platform_settings (
    id                          SMALLINT PRIMARY KEY,
    block_real_money_purchases  BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Ensure default settings row exists
INSERT INTO platform_settings (id, block_real_money_purchases, updated_at)
VALUES (1, FALSE, NOW())
ON CONFLICT (id) DO NOTHING;
