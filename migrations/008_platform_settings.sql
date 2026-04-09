-- ============================================================
-- Platform Settings
-- ============================================================

CREATE TABLE platform_settings (
    id                          SMALLINT PRIMARY KEY DEFAULT 1,
    block_real_money_purchases  BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT only_one_row CHECK (id = 1)
);

-- Insert default settings
INSERT INTO platform_settings (id, block_real_money_purchases, updated_at)
VALUES (1, FALSE, NOW())
ON CONFLICT (id) DO NOTHING;
