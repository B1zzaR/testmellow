-- ============================================================
-- VPN Platform - Production Schema
-- ============================================================

CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ─── Users ───────────────────────────────────────────────────
CREATE TABLE users (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    telegram_id         BIGINT UNIQUE,
    username            VARCHAR(64),
    email               VARCHAR(255) UNIQUE,
    password_hash       VARCHAR(255),
    yad_balance         BIGINT NOT NULL DEFAULT 0 CHECK (yad_balance >= 0),
    referrer_id         UUID REFERENCES users(id),
    referral_code       VARCHAR(32) UNIQUE NOT NULL,
    ltv                 BIGINT NOT NULL DEFAULT 0,   -- kopecks
    risk_score          SMALLINT NOT NULL DEFAULT 0 CHECK (risk_score BETWEEN 0 AND 100),
    is_admin            BOOLEAN NOT NULL DEFAULT FALSE,
    is_banned           BOOLEAN NOT NULL DEFAULT FALSE,
    remna_user_uuid     VARCHAR(64),
    device_fingerprint  VARCHAR(256),
    last_known_ip       INET,
    trial_used          BOOLEAN NOT NULL DEFAULT FALSE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_telegram_id   ON users(telegram_id);
CREATE INDEX idx_users_referral_code ON users(referral_code);
CREATE INDEX idx_users_referrer_id   ON users(referrer_id);

-- ─── Subscriptions ───────────────────────────────────────────
CREATE TABLE subscriptions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id),
    plan            VARCHAR(16) NOT NULL CHECK (plan IN ('1week','1month','3months')),
    status          VARCHAR(16) NOT NULL DEFAULT 'active'
                        CHECK (status IN ('active','expired','trial','canceled')),
    starts_at       TIMESTAMPTZ NOT NULL,
    expires_at      TIMESTAMPTZ NOT NULL,
    remna_sub_uuid  VARCHAR(64),
    paid_kopecks    BIGINT NOT NULL DEFAULT 0,
    payment_id      UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_subscriptions_user_id   ON subscriptions(user_id);
CREATE INDEX idx_subscriptions_expires_at ON subscriptions(expires_at);
CREATE INDEX idx_subscriptions_status    ON subscriptions(status);

-- ─── Payments ────────────────────────────────────────────────
-- id is Platega transactionId (UUID)
CREATE TABLE payments (
    id                  UUID PRIMARY KEY,           -- Platega transactionId
    user_id             UUID NOT NULL REFERENCES users(id),
    amount_kopecks      BIGINT NOT NULL,
    currency            VARCHAR(8) NOT NULL DEFAULT 'RUB',
    status              VARCHAR(16) NOT NULL DEFAULT 'PENDING'
                            CHECK (status IN ('PENDING','CONFIRMED','CANCELED','CHARGEBACKED')),
    plan                VARCHAR(16) NOT NULL CHECK (plan IN ('1week','1month','3months')),
    payment_method      INTEGER NOT NULL,
    platega_payload     TEXT NOT NULL DEFAULT '',
    redirect_url        TEXT NOT NULL DEFAULT '',
    webhook_received_at TIMESTAMPTZ,
    idempotency         VARCHAR(128) UNIQUE,        -- SHA256(id||status)
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payments_user_id ON payments(user_id);
CREATE INDEX idx_payments_status  ON payments(status);

-- ─── Referrals ───────────────────────────────────────────────
CREATE TABLE referrals (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    referrer_id     UUID NOT NULL REFERENCES users(id),
    referee_id      UUID NOT NULL REFERENCES users(id) UNIQUE,
    total_paid_ltv  BIGINT NOT NULL DEFAULT 0,
    total_reward    BIGINT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT no_self_referral CHECK (referrer_id <> referee_id)
);

CREATE INDEX idx_referrals_referrer ON referrals(referrer_id);

-- ─── Referral Rewards ────────────────────────────────────────
CREATE TABLE referral_rewards (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    referral_id     UUID NOT NULL REFERENCES referrals(id),
    payment_id      UUID NOT NULL REFERENCES payments(id),
    referrer_id     UUID NOT NULL REFERENCES users(id),
    amount_yad      BIGINT NOT NULL,
    immediate_yad   BIGINT NOT NULL,
    deferred_yad    BIGINT NOT NULL,
    status          VARCHAR(16) NOT NULL DEFAULT 'pending'
                        CHECK (status IN ('pending','immediate','deferred','paid','blocked')),
    risk_score      SMALLINT NOT NULL DEFAULT 0,
    scheduled_at    TIMESTAMPTZ NOT NULL,           -- 24–72 h delay
    deferred_at     TIMESTAMPTZ,                    -- 30 days after immediate
    paid_at         TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (payment_id)                             -- idempotency: one reward per payment
);

CREATE INDEX idx_ref_rewards_referrer  ON referral_rewards(referrer_id);
CREATE INDEX idx_ref_rewards_status    ON referral_rewards(status);
CREATE INDEX idx_ref_rewards_scheduled ON referral_rewards(scheduled_at) WHERE status = 'pending';

-- ─── YAD Transactions ────────────────────────────────────────
CREATE TABLE yad_transactions (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id),
    delta      BIGINT NOT NULL,
    balance    BIGINT NOT NULL,
    tx_type    VARCHAR(32) NOT NULL
                   CHECK (tx_type IN ('referral_reward','bonus','spent','promo','trial')),
    ref_id     UUID,
    note       TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_yad_tx_user_id    ON yad_transactions(user_id);
CREATE INDEX idx_yad_tx_created_at ON yad_transactions(created_at);

-- ─── Promo Codes ─────────────────────────────────────────────
CREATE TABLE promocodes (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code            VARCHAR(64) UNIQUE NOT NULL,
    yad_amount      BIGINT NOT NULL CHECK (yad_amount > 0),
    max_uses        INTEGER NOT NULL DEFAULT 1,
    used_count      INTEGER NOT NULL DEFAULT 0,
    expires_at      TIMESTAMPTZ,
    created_by_id   UUID NOT NULL REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE promocode_uses (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    promo_code_id   UUID NOT NULL REFERENCES promocodes(id),
    user_id         UUID NOT NULL REFERENCES users(id),
    used_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (promo_code_id, user_id)
);

-- ─── Tickets ─────────────────────────────────────────────────
CREATE TABLE tickets (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id),
    subject    VARCHAR(256) NOT NULL,
    status     VARCHAR(16) NOT NULL DEFAULT 'open'
                   CHECK (status IN ('open','answered','closed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tickets_user_id ON tickets(user_id);
CREATE INDEX idx_tickets_status  ON tickets(status);

CREATE TABLE ticket_messages (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_id  UUID NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    sender_id  UUID NOT NULL REFERENCES users(id),
    is_admin   BOOLEAN NOT NULL DEFAULT FALSE,
    body       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ticket_msgs_ticket ON ticket_messages(ticket_id);

-- ─── Shop ────────────────────────────────────────────────────
CREATE TABLE shop_items (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(256) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    price_yad   BIGINT NOT NULL CHECK (price_yad > 0),
    stock       INTEGER NOT NULL DEFAULT -1,    -- -1 = unlimited
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE shop_orders (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id),
    item_id    UUID NOT NULL REFERENCES shop_items(id),
    quantity   INTEGER NOT NULL DEFAULT 1 CHECK (quantity > 0),
    total_yad  BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_shop_orders_user ON shop_orders(user_id);

-- ─── Idempotency / Webhook Log ────────────────────────────────
CREATE TABLE webhook_events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source          VARCHAR(32) NOT NULL,       -- 'platega'
    external_id     VARCHAR(128) NOT NULL,      -- transactionId
    event_type      VARCHAR(64) NOT NULL,       -- 'CONFIRMED' etc.
    payload         JSONB NOT NULL,
    processed       BOOLEAN NOT NULL DEFAULT FALSE,
    processed_at    TIMESTAMPTZ,
    error           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (source, external_id, event_type)
);

-- ─── Risk Events ──────────────────────────────────────────────
CREATE TABLE risk_events (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID REFERENCES users(id),
    event_type  VARCHAR(64) NOT NULL,
    ip          INET,
    fingerprint VARCHAR(256),
    score_delta SMALLINT NOT NULL,
    details     JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_risk_events_user_id ON risk_events(user_id);

-- ─── Triggers ─────────────────────────────────────────────────
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_users_updated_at        BEFORE UPDATE ON users        FOR EACH ROW EXECUTE FUNCTION update_updated_at();
CREATE TRIGGER trg_subscriptions_updated_at BEFORE UPDATE ON subscriptions FOR EACH ROW EXECUTE FUNCTION update_updated_at();
CREATE TRIGGER trg_payments_updated_at     BEFORE UPDATE ON payments     FOR EACH ROW EXECUTE FUNCTION update_updated_at();
CREATE TRIGGER trg_tickets_updated_at      BEFORE UPDATE ON tickets      FOR EACH ROW EXECUTE FUNCTION update_updated_at();
