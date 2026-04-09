package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RunMigrations applies any pending SQL migrations tracked via the
// schema_migrations table. Migrations are idempotent: each version string is
// only applied once.
func RunMigrations(ctx context.Context, db *pgxpool.Pool) error {
	// Bootstrap the tracking table
	_, err := db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    VARCHAR(128) PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`)
	if err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	migrations := []struct {
		version string
		sql     string
	}{
		{
			version: "001_bootstrap_schema",
			sql: `
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS users (
	id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	telegram_id             BIGINT UNIQUE,
	username                VARCHAR(64),
	email                   VARCHAR(255) UNIQUE,
	password_hash           VARCHAR(255),
	yad_balance             BIGINT NOT NULL DEFAULT 0 CHECK (yad_balance >= 0),
	referrer_id             UUID REFERENCES users(id),
	referral_code           VARCHAR(32) UNIQUE NOT NULL,
	ltv                     BIGINT NOT NULL DEFAULT 0,
	risk_score              SMALLINT NOT NULL DEFAULT 0 CHECK (risk_score BETWEEN 0 AND 100),
	is_admin                BOOLEAN NOT NULL DEFAULT FALSE,
	is_banned               BOOLEAN NOT NULL DEFAULT FALSE,
	remna_user_uuid         VARCHAR(64),
	device_fingerprint      VARCHAR(256),
	last_known_ip           INET,
	trial_used              BOOLEAN NOT NULL DEFAULT FALSE,
	active_discount_code    VARCHAR(64),
	active_discount_percent SMALLINT NOT NULL DEFAULT 0 CHECK (active_discount_percent BETWEEN 0 AND 100),
	created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_telegram_id   ON users(telegram_id);
CREATE INDEX IF NOT EXISTS idx_users_referral_code ON users(referral_code);
CREATE INDEX IF NOT EXISTS idx_users_referrer_id   ON users(referrer_id);

CREATE TABLE IF NOT EXISTS subscriptions (
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

CREATE INDEX IF NOT EXISTS idx_subscriptions_user_id    ON subscriptions(user_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_expires_at ON subscriptions(expires_at);
CREATE INDEX IF NOT EXISTS idx_subscriptions_status     ON subscriptions(status);

CREATE TABLE IF NOT EXISTS payments (
	id                  UUID PRIMARY KEY,
	user_id             UUID NOT NULL REFERENCES users(id),
	amount_kopecks      BIGINT NOT NULL,
	currency            VARCHAR(8) NOT NULL DEFAULT 'RUB',
	status              VARCHAR(16) NOT NULL DEFAULT 'PENDING'
							CHECK (status IN ('PENDING','CONFIRMED','CANCELED','CHARGEBACKED','EXPIRED')),
	plan                VARCHAR(16) NOT NULL CHECK (plan IN ('1week','1month','3months')),
	payment_method      INTEGER NOT NULL,
	platega_payload     TEXT NOT NULL DEFAULT '',
	redirect_url        TEXT NOT NULL DEFAULT '',
	webhook_received_at TIMESTAMPTZ,
	idempotency         VARCHAR(128) UNIQUE,
	expires_at          TIMESTAMPTZ,
	created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_payments_user_id ON payments(user_id);
CREATE INDEX IF NOT EXISTS idx_payments_status  ON payments(status);

CREATE TABLE IF NOT EXISTS referrals (
	id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	referrer_id     UUID NOT NULL REFERENCES users(id),
	referee_id      UUID NOT NULL REFERENCES users(id) UNIQUE,
	total_paid_ltv  BIGINT NOT NULL DEFAULT 0,
	total_reward    BIGINT NOT NULL DEFAULT 0,
	created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	CONSTRAINT no_self_referral CHECK (referrer_id <> referee_id)
);

CREATE INDEX IF NOT EXISTS idx_referrals_referrer ON referrals(referrer_id);

CREATE TABLE IF NOT EXISTS referral_rewards (
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
	scheduled_at    TIMESTAMPTZ NOT NULL,
	deferred_at     TIMESTAMPTZ,
	paid_at         TIMESTAMPTZ,
	created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	UNIQUE (payment_id)
);

CREATE INDEX IF NOT EXISTS idx_ref_rewards_referrer  ON referral_rewards(referrer_id);
CREATE INDEX IF NOT EXISTS idx_ref_rewards_status    ON referral_rewards(status);
CREATE INDEX IF NOT EXISTS idx_ref_rewards_scheduled ON referral_rewards(scheduled_at) WHERE status = 'pending';

CREATE TABLE IF NOT EXISTS yad_transactions (
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

CREATE INDEX IF NOT EXISTS idx_yad_tx_user_id    ON yad_transactions(user_id);
CREATE INDEX IF NOT EXISTS idx_yad_tx_created_at ON yad_transactions(created_at);

CREATE TABLE IF NOT EXISTS promocodes (
	id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	code            VARCHAR(64) UNIQUE NOT NULL,
	yad_amount      BIGINT NOT NULL CHECK (yad_amount >= 0),
	max_uses        INTEGER NOT NULL DEFAULT 1,
	used_count      INTEGER NOT NULL DEFAULT 0,
	expires_at      TIMESTAMPTZ,
	created_by_id   UUID NOT NULL REFERENCES users(id),
	promo_type      VARCHAR(16) NOT NULL DEFAULT 'yad' CHECK (promo_type IN ('yad', 'discount')),
	discount_percent SMALLINT NOT NULL DEFAULT 0 CHECK (discount_percent BETWEEN 0 AND 100),
	created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS promocode_uses (
	id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	promo_code_id   UUID NOT NULL REFERENCES promocodes(id),
	user_id         UUID NOT NULL REFERENCES users(id),
	used_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	UNIQUE (promo_code_id, user_id)
);

CREATE TABLE IF NOT EXISTS tickets (
	id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	user_id    UUID NOT NULL REFERENCES users(id),
	subject    VARCHAR(256) NOT NULL,
	status     VARCHAR(16) NOT NULL DEFAULT 'open'
				   CHECK (status IN ('open','answered','closed')),
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tickets_user_id ON tickets(user_id);
CREATE INDEX IF NOT EXISTS idx_tickets_status  ON tickets(status);

CREATE TABLE IF NOT EXISTS ticket_messages (
	id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	ticket_id  UUID NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
	sender_id  UUID NOT NULL REFERENCES users(id),
	is_admin   BOOLEAN NOT NULL DEFAULT FALSE,
	body       TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ticket_msgs_ticket ON ticket_messages(ticket_id);

CREATE TABLE IF NOT EXISTS shop_items (
	id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	name        VARCHAR(256) NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	price_yad   BIGINT NOT NULL CHECK (price_yad > 0),
	stock       INTEGER NOT NULL DEFAULT -1,
	is_active   BOOLEAN NOT NULL DEFAULT TRUE,
	created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS shop_orders (
	id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	user_id    UUID NOT NULL REFERENCES users(id),
	item_id    UUID NOT NULL REFERENCES shop_items(id),
	quantity   INTEGER NOT NULL DEFAULT 1 CHECK (quantity > 0),
	total_yad  BIGINT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_shop_orders_user ON shop_orders(user_id);

CREATE TABLE IF NOT EXISTS webhook_events (
	id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	source          VARCHAR(32) NOT NULL,
	external_id     VARCHAR(128) NOT NULL,
	event_type      VARCHAR(64) NOT NULL,
	payload         JSONB NOT NULL,
	processed       BOOLEAN NOT NULL DEFAULT FALSE,
	processed_at    TIMESTAMPTZ,
	error           TEXT,
	created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	UNIQUE (source, external_id, event_type)
);

CREATE TABLE IF NOT EXISTS risk_events (
	id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	user_id     UUID REFERENCES users(id),
	event_type  VARCHAR(64) NOT NULL,
	ip          INET,
	fingerprint VARCHAR(256),
	score_delta SMALLINT NOT NULL,
	details     JSONB,
	created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_risk_events_user_id ON risk_events(user_id);
`,
		},
		{
			version: "002_payment_expires",
			sql: `
ALTER TABLE payments ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ;
ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_status_check;
ALTER TABLE payments ADD CONSTRAINT payments_status_check
    CHECK (status IN ('PENDING','CONFIRMED','CANCELED','CHARGEBACKED','EXPIRED'));
CREATE INDEX IF NOT EXISTS idx_payments_user_pending
    ON payments(user_id, created_at DESC)
    WHERE status = 'PENDING';
`,
		},
		{
			version: "003_admin_audit_logs",
			sql: `
CREATE TABLE IF NOT EXISTS admin_audit_logs (
    id          UUID        PRIMARY KEY,
    admin_id    UUID        NOT NULL REFERENCES users(id) ON DELETE SET NULL,
    action      VARCHAR(128) NOT NULL,
    target_type VARCHAR(64),
    target_id   UUID,
    details     TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_audit_admin_id   ON admin_audit_logs(admin_id,    created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_target     ON admin_audit_logs(target_type, target_id);
CREATE INDEX IF NOT EXISTS idx_audit_created_at ON admin_audit_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_payments_status  ON payments(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_payments_user_id ON payments(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_subs_status      ON subscriptions(status, expires_at DESC);
CREATE INDEX IF NOT EXISTS idx_subs_user_id     ON subscriptions(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_yad_user_id      ON yad_transactions(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_referrals_referrer ON referrals(referrer_id, created_at DESC);
`,
		},
		{
			version: "004_discount_promo",
			sql: `
ALTER TABLE promocodes
  ADD COLUMN IF NOT EXISTS promo_type VARCHAR(16) NOT NULL DEFAULT 'yad'
    CHECK (promo_type IN ('yad', 'discount'));
ALTER TABLE promocodes
  ADD COLUMN IF NOT EXISTS discount_percent SMALLINT NOT NULL DEFAULT 0
    CHECK (discount_percent BETWEEN 0 AND 100);
ALTER TABLE promocodes DROP CONSTRAINT IF EXISTS promocodes_yad_amount_check;
ALTER TABLE promocodes ADD CONSTRAINT promocodes_yad_amount_check
  CHECK (yad_amount >= 0);
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS active_discount_code    VARCHAR(64);
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS active_discount_percent SMALLINT NOT NULL DEFAULT 0
    CHECK (active_discount_percent BETWEEN 0 AND 100);
`,
		},
		{
			version: "005_devices",
			sql: `
CREATE TABLE IF NOT EXISTS devices (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_name TEXT        NOT NULL DEFAULT 'Unknown Device',
    last_active TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    is_active   BOOLEAN     NOT NULL DEFAULT TRUE
);
CREATE INDEX IF NOT EXISTS idx_devices_user_id     ON devices(user_id);
CREATE INDEX IF NOT EXISTS idx_devices_last_active ON devices(last_active);
`,
		},
		{
			version: "006_updated_at_triggers",
			sql: `
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;

DO $$ BEGIN
    CREATE TRIGGER trg_users_updated_at
        BEFORE UPDATE ON users
        FOR EACH ROW EXECUTE FUNCTION set_updated_at();
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TRIGGER trg_subscriptions_updated_at
        BEFORE UPDATE ON subscriptions
        FOR EACH ROW EXECUTE FUNCTION set_updated_at();
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TRIGGER trg_payments_updated_at
        BEFORE UPDATE ON payments
        FOR EACH ROW EXECUTE FUNCTION set_updated_at();
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TRIGGER trg_tickets_updated_at
        BEFORE UPDATE ON tickets
        FOR EACH ROW EXECUTE FUNCTION set_updated_at();
EXCEPTION WHEN duplicate_object THEN NULL; END $$;
`,
		},
		{
			version: "007_username_unique",
			sql: `
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users(username)
    WHERE username IS NOT NULL;
`,
		},
		{
			version: "008_platform_settings",
			sql: `
CREATE TABLE IF NOT EXISTS platform_settings (
    id                          SMALLINT PRIMARY KEY,
    block_real_money_purchases  BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO platform_settings (id, block_real_money_purchases, updated_at)
VALUES (1, FALSE, NOW())
ON CONFLICT (id) DO NOTHING;
`,
		},
	}

	for _, m := range migrations {
		var count int
		_ = db.QueryRow(ctx,
			`SELECT COUNT(*) FROM schema_migrations WHERE version=$1`, m.version,
		).Scan(&count)
		if count > 0 {
			continue // already applied
		}

		if _, err := db.Exec(ctx, m.sql); err != nil {
			return fmt.Errorf("migration %s: %w", m.version, err)
		}
		if _, err := db.Exec(ctx,
			`INSERT INTO schema_migrations (version) VALUES ($1)`, m.version,
		); err != nil {
			return fmt.Errorf("record migration %s: %w", m.version, err)
		}
	}
	return nil
}
