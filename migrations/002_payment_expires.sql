-- ============================================================
-- Migration 002: add expires_at to payments + EXPIRED status
-- ============================================================

-- Add expires_at column (Platega links expire in 15 minutes)
ALTER TABLE payments ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ;

-- Drop the old status CHECK constraint (auto-named by PostgreSQL)
ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_status_check;

-- Re-add constraint with EXPIRED status
ALTER TABLE payments ADD CONSTRAINT payments_status_check
    CHECK (status IN ('PENDING','CONFIRMED','CANCELED','CHARGEBACKED','EXPIRED'));

-- Index for efficiently querying a user's pending payments
CREATE INDEX IF NOT EXISTS idx_payments_user_pending
    ON payments(user_id, created_at DESC)
    WHERE status = 'PENDING';
