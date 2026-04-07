// Package postgres – admin-specific repository methods.
// All methods are added to the existing UserRepo struct so no new wiring is needed.
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/vpnplatform/internal/domain"
)

// ─── Payments ────────────────────────────────────────────────────────────────

// ListAllPayments returns all payments with optional filters.
func (r *UserRepo) ListAllPayments(ctx context.Context, status string, from, to *time.Time, limit, offset int) ([]*domain.Payment, error) {
	q := `SELECT id, user_id, amount_kopecks, currency, status, plan, payment_method,
	             redirect_url, expires_at, created_at, updated_at
	      FROM payments WHERE 1=1`
	args := []any{}
	n := 1

	if status != "" {
		q += fmt.Sprintf(" AND status = $%d", n)
		args = append(args, status)
		n++
	}
	if from != nil {
		q += fmt.Sprintf(" AND created_at >= $%d", n)
		args = append(args, *from)
		n++
	}
	if to != nil {
		q += fmt.Sprintf(" AND created_at <= $%d", n)
		args = append(args, *to)
		n++
	}
	q += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", n, n+1)
	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var payments []*domain.Payment
	for rows.Next() {
		p := &domain.Payment{}
		if err := rows.Scan(
			&p.ID, &p.UserID, &p.AmountKopecks, &p.Currency, &p.Status, &p.Plan, &p.PaymentMethod,
			&p.RedirectURL, &p.ExpiresAt, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		payments = append(payments, p)
	}
	return payments, rows.Err()
}

// ListPaymentsByUser returns the most recent payments for a specific user (admin view).
func (r *UserRepo) ListPaymentsByUser(ctx context.Context, userID uuid.UUID, limit int) ([]*domain.Payment, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, amount_kopecks, currency, status, plan, payment_method,
		       redirect_url, expires_at, created_at, updated_at
		FROM payments WHERE user_id=$1 ORDER BY created_at DESC LIMIT $2`,
		userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var payments []*domain.Payment
	for rows.Next() {
		p := &domain.Payment{}
		if err := rows.Scan(
			&p.ID, &p.UserID, &p.AmountKopecks, &p.Currency, &p.Status, &p.Plan, &p.PaymentMethod,
			&p.RedirectURL, &p.ExpiresAt, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		payments = append(payments, p)
	}
	return payments, rows.Err()
}

// ─── Subscriptions ────────────────────────────────────────────────────────────

// ListAllSubscriptions returns all subscriptions with optional filters.
func (r *UserRepo) ListAllSubscriptions(ctx context.Context, status string, userID *uuid.UUID, limit, offset int) ([]*domain.Subscription, error) {
	q := `SELECT id, user_id, plan, status, starts_at, expires_at, remna_sub_uuid, paid_kopecks, payment_id, created_at, updated_at
	      FROM subscriptions WHERE 1=1`
	args := []any{}
	n := 1

	if status != "" {
		q += fmt.Sprintf(" AND status = $%d", n)
		args = append(args, status)
		n++
	}
	if userID != nil {
		q += fmt.Sprintf(" AND user_id = $%d", n)
		args = append(args, *userID)
		n++
	}
	q += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", n, n+1)
	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []*domain.Subscription
	for rows.Next() {
		s := &domain.Subscription{}
		if err := rows.Scan(&s.ID, &s.UserID, &s.Plan, &s.Status, &s.StartsAt, &s.ExpiresAt,
			&s.RemnaSubUUID, &s.PaidKopecks, &s.PaymentID, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		subs = append(subs, s)
	}
	return subs, rows.Err()
}

// SetSubscriptionStatus updates the status of a subscription.
func (r *UserRepo) SetSubscriptionStatus(ctx context.Context, subID uuid.UUID, status domain.SubscriptionStatus) error {
	_, err := r.db.Exec(ctx,
		`UPDATE subscriptions SET status=$1, updated_at=NOW() WHERE id=$2`, status, subID)
	return err
}

// ExtendSubscriptionByDays adds N days to a subscription, reactivating it if expired/canceled.
func (r *UserRepo) ExtendSubscriptionByDays(ctx context.Context, subID uuid.UUID, days int) (*domain.Subscription, error) {
	s := &domain.Subscription{}
	err := r.db.QueryRow(ctx, `
		UPDATE subscriptions
		SET expires_at  = GREATEST(expires_at, NOW()) + ($1 || ' days')::interval,
		    status      = CASE WHEN status IN ('expired','canceled') THEN 'active' ELSE status END,
		    updated_at  = NOW()
		WHERE id = $2
		RETURNING id, user_id, plan, status, starts_at, expires_at, remna_sub_uuid, paid_kopecks, payment_id, created_at, updated_at`,
		days, subID).Scan(
		&s.ID, &s.UserID, &s.Plan, &s.Status, &s.StartsAt, &s.ExpiresAt,
		&s.RemnaSubUUID, &s.PaidKopecks, &s.PaymentID, &s.CreatedAt, &s.UpdatedAt,
	)
	return s, err
}

// ─── YAD Transactions ─────────────────────────────────────────────────────────

// ListAllYADTransactions returns YAD transactions with optional user/type filters.
func (r *UserRepo) ListAllYADTransactions(ctx context.Context, userID *uuid.UUID, txType string, limit, offset int) ([]*domain.YADTransaction, error) {
	q := `SELECT id, user_id, delta, balance, tx_type, ref_id, note, created_at
	      FROM yad_transactions WHERE 1=1`
	args := []any{}
	n := 1

	if userID != nil {
		q += fmt.Sprintf(" AND user_id = $%d", n)
		args = append(args, *userID)
		n++
	}
	if txType != "" {
		q += fmt.Sprintf(" AND tx_type = $%d", n)
		args = append(args, txType)
		n++
	}
	q += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", n, n+1)
	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var txs []*domain.YADTransaction
	for rows.Next() {
		t := &domain.YADTransaction{}
		if err := rows.Scan(&t.ID, &t.UserID, &t.Delta, &t.Balance, &t.TxType, &t.RefID, &t.Note, &t.CreatedAt); err != nil {
			return nil, err
		}
		txs = append(txs, t)
	}
	return txs, rows.Err()
}

// ─── Referrals ────────────────────────────────────────────────────────────────

// GetAllReferrals returns all referral relationships with optional referrer filter.
func (r *UserRepo) GetAllReferrals(ctx context.Context, referrerID *uuid.UUID, limit, offset int) ([]*domain.Referral, error) {
	q := `SELECT id, referrer_id, referee_id, total_paid_ltv, total_reward, created_at
	      FROM referrals WHERE 1=1`
	args := []any{}
	n := 1

	if referrerID != nil {
		q += fmt.Sprintf(" AND referrer_id = $%d", n)
		args = append(args, *referrerID)
		n++
	}
	q += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", n, n+1)
	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var refs []*domain.Referral
	for rows.Next() {
		ref := &domain.Referral{}
		if err := rows.Scan(&ref.ID, &ref.ReferrerID, &ref.RefereeID,
			&ref.TotalPaidLTV, &ref.TotalReward, &ref.CreatedAt); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, rows.Err()
}

// ─── Audit Logs ───────────────────────────────────────────────────────────────

// CreateAuditLog persists a new admin action record.
func (r *UserRepo) CreateAuditLog(ctx context.Context, entry *domain.AdminAuditLog) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO admin_audit_logs (id, admin_id, action, target_type, target_id, details, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		entry.ID, entry.AdminID, entry.Action, entry.TargetType, entry.TargetID, entry.Details, entry.CreatedAt,
	)
	return err
}

// ListAuditLogs returns recent audit log entries joined with admin user info.
func (r *UserRepo) ListAuditLogs(ctx context.Context, limit, offset int) ([]*domain.AdminAuditLog, error) {
	rows, err := r.db.Query(ctx, `
		SELECT al.id, al.admin_id, al.action, al.target_type, al.target_id, al.details, al.created_at,
		       u.username, u.email
		FROM admin_audit_logs al
		LEFT JOIN users u ON u.id = al.admin_id
		ORDER BY al.created_at DESC LIMIT $1 OFFSET $2`,
		limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*domain.AdminAuditLog
	for rows.Next() {
		l := &domain.AdminAuditLog{}
		if err := rows.Scan(&l.ID, &l.AdminID, &l.Action, &l.TargetType, &l.TargetID,
			&l.Details, &l.CreatedAt, &l.AdminUsername, &l.AdminEmail); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

// ─── Analytics ────────────────────────────────────────────────────────────────

// GetRevenueByDay returns daily revenue statistics for the past N days.
func (r *UserRepo) GetRevenueByDay(ctx context.Context, days int) ([]domain.RevenueStat, error) {
	rows, err := r.db.Query(ctx, `
		SELECT date_trunc('day', created_at)::date AS day,
		       COALESCE(SUM(amount_kopecks), 0)    AS total_kopecks,
		       COUNT(*)                             AS count
		FROM payments
		WHERE status = 'CONFIRMED'
		  AND created_at >= NOW() - ($1 || ' days')::interval
		GROUP BY day
		ORDER BY day ASC`,
		days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []domain.RevenueStat
	for rows.Next() {
		var s domain.RevenueStat
		if err := rows.Scan(&s.Date, &s.TotalKopecks, &s.Count); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

// GetTopReferrers returns users with highest referral reward earnings.
func (r *UserRepo) GetTopReferrers(ctx context.Context, limit int) ([]domain.TopReferrer, error) {
	rows, err := r.db.Query(ctx, `
		SELECT r.referrer_id,
		       u.username,
		       u.email,
		       COUNT(r.id)           AS referral_count,
		       COALESCE(SUM(r.total_reward), 0) AS total_reward_yad
		FROM referrals r
		LEFT JOIN users u ON u.id = r.referrer_id
		GROUP BY r.referrer_id, u.username, u.email
		ORDER BY total_reward_yad DESC LIMIT $1`,
		limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domain.TopReferrer
	for rows.Next() {
		var tr domain.TopReferrer
		if err := rows.Scan(&tr.UserID, &tr.Username, &tr.Email, &tr.ReferralCount, &tr.TotalRewardYAD); err != nil {
			return nil, err
		}
		result = append(result, tr)
	}
	return result, rows.Err()
}
