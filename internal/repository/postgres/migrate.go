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
