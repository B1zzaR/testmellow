-- ============================================================
-- Migration 003: discount promo codes + user active discount
-- ============================================================

-- Add promo_type column (yad = credits, discount = % off subscription)
ALTER TABLE promocodes
  ADD COLUMN IF NOT EXISTS promo_type VARCHAR(16) NOT NULL DEFAULT 'yad'
    CHECK (promo_type IN ('yad', 'discount'));

-- Add discount_percent (0 for yad codes)
ALTER TABLE promocodes
  ADD COLUMN IF NOT EXISTS discount_percent SMALLINT NOT NULL DEFAULT 0
    CHECK (discount_percent BETWEEN 0 AND 100);

-- Relax yad_amount constraint to allow 0 for discount codes
ALTER TABLE promocodes DROP CONSTRAINT IF EXISTS promocodes_yad_amount_check;
ALTER TABLE promocodes ADD CONSTRAINT promocodes_yad_amount_check
  CHECK (yad_amount >= 0);

-- Store pending discount on user until they purchase a subscription
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS active_discount_code    VARCHAR(64);
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS active_discount_percent SMALLINT NOT NULL DEFAULT 0
    CHECK (active_discount_percent BETWEEN 0 AND 100);
