package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vpnplatform/internal/domain"
)

type UserRepo struct {
	db *pgxpool.Pool
}

func NewUserRepo(db *pgxpool.Pool) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) Create(ctx context.Context, u *domain.User) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO users (
			id, telegram_id, username, email, password_hash,
			yad_balance, referrer_id, referral_code, ltv,
			risk_score, is_admin, is_banned, remna_user_uuid,
			device_fingerprint, last_known_ip, trial_used,
			created_at, updated_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18
		)`,
		u.ID, u.TelegramID, u.Username, u.Email, u.PasswordHash,
		u.YADBalance, u.ReferrerID, u.ReferralCode, u.LTV,
		u.RiskScore, u.IsAdmin, u.IsBanned, u.RemnaUserUUID,
		u.DeviceFingerprint, u.LastKnownIP, u.TrialUsed,
		u.CreatedAt, u.UpdatedAt,
	)
	return err
}

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	u := &domain.User{}
	err := r.db.QueryRow(ctx, `
		SELECT id, telegram_id, username, email, password_hash,
		       yad_balance, referrer_id, referral_code, ltv,
		       risk_score, is_admin, is_banned, remna_user_uuid,
		       device_fingerprint, last_known_ip::text, trial_used,
		       created_at, updated_at
		FROM users WHERE id = $1`, id).Scan(
		&u.ID, &u.TelegramID, &u.Username, &u.Email, &u.PasswordHash,
		&u.YADBalance, &u.ReferrerID, &u.ReferralCode, &u.LTV,
		&u.RiskScore, &u.IsAdmin, &u.IsBanned, &u.RemnaUserUUID,
		&u.DeviceFingerprint, &u.LastKnownIP, &u.TrialUsed,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return u, err
}

func (r *UserRepo) GetByTelegramID(ctx context.Context, tgID int64) (*domain.User, error) {
	u := &domain.User{}
	err := r.db.QueryRow(ctx, `
		SELECT id, telegram_id, username, email, password_hash,
		       yad_balance, referrer_id, referral_code, ltv,
		       risk_score, is_admin, is_banned, remna_user_uuid,
		       device_fingerprint, last_known_ip::text, trial_used,
		       created_at, updated_at
		FROM users WHERE telegram_id = $1`, tgID).Scan(
		&u.ID, &u.TelegramID, &u.Username, &u.Email, &u.PasswordHash,
		&u.YADBalance, &u.ReferrerID, &u.ReferralCode, &u.LTV,
		&u.RiskScore, &u.IsAdmin, &u.IsBanned, &u.RemnaUserUUID,
		&u.DeviceFingerprint, &u.LastKnownIP, &u.TrialUsed,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return u, err
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	u := &domain.User{}
	err := r.db.QueryRow(ctx, `
		SELECT id, telegram_id, username, email, password_hash,
		       yad_balance, referrer_id, referral_code, ltv,
		       risk_score, is_admin, is_banned, remna_user_uuid,
		       device_fingerprint, last_known_ip::text, trial_used,
		       created_at, updated_at
		FROM users WHERE email = $1`, email).Scan(
		&u.ID, &u.TelegramID, &u.Username, &u.Email, &u.PasswordHash,
		&u.YADBalance, &u.ReferrerID, &u.ReferralCode, &u.LTV,
		&u.RiskScore, &u.IsAdmin, &u.IsBanned, &u.RemnaUserUUID,
		&u.DeviceFingerprint, &u.LastKnownIP, &u.TrialUsed,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return u, err
}

func (r *UserRepo) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	u := &domain.User{}
	err := r.db.QueryRow(ctx, `
		SELECT id, telegram_id, username, email, password_hash,
		       yad_balance, referrer_id, referral_code, ltv,
		       risk_score, is_admin, is_banned, remna_user_uuid,
		       device_fingerprint, last_known_ip::text, trial_used,
		       created_at, updated_at
		FROM users WHERE username = $1`, username).Scan(
		&u.ID, &u.TelegramID, &u.Username, &u.Email, &u.PasswordHash,
		&u.YADBalance, &u.ReferrerID, &u.ReferralCode, &u.LTV,
		&u.RiskScore, &u.IsAdmin, &u.IsBanned, &u.RemnaUserUUID,
		&u.DeviceFingerprint, &u.LastKnownIP, &u.TrialUsed,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return u, err
}

func (r *UserRepo) GetByReferralCode(ctx context.Context, code string) (*domain.User, error) {
	u := &domain.User{}
	err := r.db.QueryRow(ctx, `
		SELECT id, telegram_id, username, email, password_hash,
		       yad_balance, referrer_id, referral_code, ltv,
		       risk_score, is_admin, is_banned, remna_user_uuid,
		       device_fingerprint, last_known_ip::text, trial_used,
		       created_at, updated_at
		FROM users WHERE referral_code = $1`, code).Scan(
		&u.ID, &u.TelegramID, &u.Username, &u.Email, &u.PasswordHash,
		&u.YADBalance, &u.ReferrerID, &u.ReferralCode, &u.LTV,
		&u.RiskScore, &u.IsAdmin, &u.IsBanned, &u.RemnaUserUUID,
		&u.DeviceFingerprint, &u.LastKnownIP, &u.TrialUsed,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return u, err
}

// UpdateYADBalance atomically adjusts balance and records YAD transaction.
// Must be called inside a serializable transaction for correctness.
func (r *UserRepo) AdjustYADBalance(ctx context.Context, tx pgx.Tx, userID uuid.UUID, delta int64, txType domain.YADTxType, refID *uuid.UUID, note string) error {
	var newBalance int64
	err := tx.QueryRow(ctx, `
		UPDATE users SET yad_balance = yad_balance + $1, updated_at = NOW()
		WHERE id = $2 AND (yad_balance + $1) >= 0
		RETURNING yad_balance`, delta, userID).Scan(&newBalance)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("insufficient YAD balance or user not found")
	}
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO yad_transactions (id, user_id, delta, balance, tx_type, ref_id, note, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,NOW())`,
		uuid.New(), userID, delta, newBalance, txType, refID, note,
	)
	return err
}

func (r *UserRepo) UpdateRiskScore(ctx context.Context, userID uuid.UUID, score int) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET risk_score = $1, updated_at = NOW() WHERE id = $2`, score, userID)
	return err
}

func (r *UserRepo) UpdateRemnaUUID(ctx context.Context, userID uuid.UUID, remnaUUID string) error {
	if remnaUUID == "" {
		return nil // don't overwrite with empty
	}
	_, err := r.db.Exec(ctx, `UPDATE users SET remna_user_uuid = $1, updated_at = NOW() WHERE id = $2`, remnaUUID, userID)
	return err
}

func (r *UserRepo) UpdateLTV(ctx context.Context, tx pgx.Tx, userID uuid.UUID, addKopecks int64) error {
	_, err := tx.Exec(ctx, `UPDATE users SET ltv = ltv + $1, updated_at = NOW() WHERE id = $2`, addKopecks, userID)
	return err
}

func (r *UserRepo) SetTrialUsed(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET trial_used = TRUE, updated_at = NOW() WHERE id = $1`, userID)
	return err
}

func (r *UserRepo) List(ctx context.Context, limit, offset int) ([]*domain.User, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, telegram_id, username, email, password_hash,
		       yad_balance, referrer_id, referral_code, ltv,
		       risk_score, is_admin, is_banned, remna_user_uuid,
		       device_fingerprint, last_known_ip::text, trial_used,
		       created_at, updated_at
		FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		u := &domain.User{}
		if err := rows.Scan(
			&u.ID, &u.TelegramID, &u.Username, &u.Email, &u.PasswordHash,
			&u.YADBalance, &u.ReferrerID, &u.ReferralCode, &u.LTV,
			&u.RiskScore, &u.IsAdmin, &u.IsBanned, &u.RemnaUserUUID,
			&u.DeviceFingerprint, &u.LastKnownIP, &u.TrialUsed,
			&u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (r *UserRepo) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
}

// UpdateFingerprint stores the latest device fingerprint and IP
func (r *UserRepo) UpdateFingerprint(ctx context.Context, userID uuid.UUID, fp, ip string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE users SET device_fingerprint=$1, last_known_ip=$2::inet, updated_at=NOW() WHERE id=$3`,
		fp, ip, userID)
	return err
}

// BanUser bans a user
func (r *UserRepo) BanUser(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET is_banned=TRUE, updated_at=NOW() WHERE id=$1`, userID)
	return err
}

// GetLTV returns the current LTV for a user
func (r *UserRepo) GetLTV(ctx context.Context, userID uuid.UUID) (int64, error) {
	var ltv int64
	err := r.db.QueryRow(ctx, `SELECT ltv FROM users WHERE id=$1`, userID).Scan(&ltv)
	return ltv, err
}

// GetPaymentHistory returns paginated payments for a user and the total count.
func (r *UserRepo) GetPaymentHistory(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.Payment, int, error) {
	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM payments WHERE user_id = $1`, userID).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, amount_kopecks, currency, status, plan, payment_method,
		       redirect_url, expires_at, created_at, updated_at
		FROM payments WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var payments []*domain.Payment
	for rows.Next() {
		p := &domain.Payment{}
		if err := rows.Scan(
			&p.ID, &p.UserID, &p.AmountKopecks, &p.Currency, &p.Status, &p.Plan, &p.PaymentMethod,
			&p.RedirectURL, &p.ExpiresAt, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		payments = append(payments, p)
	}
	return payments, total, rows.Err()
}

// GetExpiredSubscriptionsUserIDs lists user IDs with active/trial subs that have just expired
func (r *UserRepo) GetExpiredSubscriptionUserIDs(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT user_id FROM subscriptions
		WHERE status IN ('active','trial') AND expires_at <= NOW()`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// SetPassword updates password hash for a user
func (r *UserRepo) SetPassword(ctx context.Context, userID uuid.UUID, hash string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE users SET password_hash=$1, updated_at=NOW() WHERE id=$2`, hash, userID)
	return err
}

// GetYADTransactions lists recent YAD transactions for a user
func (r *UserRepo) GetYADTransactions(ctx context.Context, userID uuid.UUID, limit int) ([]*domain.YADTransaction, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, delta, balance, tx_type, ref_id, note, created_at
		FROM yad_transactions WHERE user_id=$1 ORDER BY created_at DESC LIMIT $2`,
		userID, limit)
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

// UpdateBanStatus sets the banned flag
func (r *UserRepo) UpdateBanStatus(ctx context.Context, userID uuid.UUID, banned bool) error {
	_, err := r.db.Exec(ctx,
		`UPDATE users SET is_banned=$1, updated_at=NOW() WHERE id=$2`, banned, userID)
	return err
}

// SetAdmin grants or revokes admin privileges for a user.
func (r *UserRepo) SetAdmin(ctx context.Context, userID uuid.UUID, isAdmin bool) error {
	_, err := r.db.Exec(ctx,
		`UPDATE users SET is_admin=$1, updated_at=NOW() WHERE id=$2`, isAdmin, userID)
	return err
}

// SetTelegramID sets or clears the Telegram ID for a user.
// Pass nil to unlink.
func (r *UserRepo) SetTelegramID(ctx context.Context, userID uuid.UUID, tgID *int64) error {
	_, err := r.db.Exec(ctx,
		`UPDATE users SET telegram_id=$1, updated_at=NOW() WHERE id=$2`, tgID, userID)
	return err
}

// CheckIPExists checks if any other user registered from the same IP
func (r *UserRepo) CountUsersFromIP(ctx context.Context, ip string, excludeUserID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM users WHERE last_known_ip=$1::inet AND id<>$2`, ip, excludeUserID).Scan(&count)
	return count, err
}

// GetExpiringSubscriptions returns subscriptions expiring within the next 24 hours
func (r *UserRepo) GetExpiringSubscriptions(ctx context.Context) ([]*domain.Subscription, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, plan, status, starts_at, expires_at, remna_sub_uuid, paid_kopecks, payment_id, created_at, updated_at
		FROM subscriptions
		WHERE status='active' AND expires_at BETWEEN NOW() AND NOW() + INTERVAL '24 hours'`)
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

// GetDailyYADCredit returns total YAD credited to a user today
func (r *UserRepo) GetDailyYADCredit(ctx context.Context, userID uuid.UUID) (int64, error) {
	var total int64
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(delta),0) FROM yad_transactions
		WHERE user_id=$1 AND delta>0 AND created_at >= date_trunc('day', NOW())`,
		userID).Scan(&total)
	return total, err
}

// GetDailyReferralCount returns number of referrals registered today for a referrer
func (r *UserRepo) GetDailyReferralCount(ctx context.Context, referrerID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM referrals
		WHERE referrer_id=$1 AND created_at >= date_trunc('day', NOW())`,
		referrerID).Scan(&count)
	return count, err
}

// GetReferral returns the referral relationship  for a referee
func (r *UserRepo) GetReferralByReferee(ctx context.Context, refereeID uuid.UUID) (*domain.Referral, error) {
	ref := &domain.Referral{}
	err := r.db.QueryRow(ctx, `
		SELECT id, referrer_id, referee_id, total_paid_ltv, total_reward, created_at
		FROM referrals WHERE referee_id=$1`, refereeID).Scan(
		&ref.ID, &ref.ReferrerID, &ref.RefereeID, &ref.TotalPaidLTV, &ref.TotalReward, &ref.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return ref, err
}

// GetReferralsByReferrer returns all referral relationships where the user is referrer
func (r *UserRepo) GetReferralsByReferrer(ctx context.Context, referrerID uuid.UUID) ([]*domain.Referral, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, referrer_id, referee_id, total_paid_ltv, total_reward, created_at
		FROM referrals WHERE referrer_id=$1 ORDER BY created_at DESC`, referrerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var refs []*domain.Referral
	for rows.Next() {
		ref := &domain.Referral{}
		if err := rows.Scan(&ref.ID, &ref.ReferrerID, &ref.RefereeID, &ref.TotalPaidLTV, &ref.TotalReward, &ref.CreatedAt); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, rows.Err()
}

// CreateReferral inserts the referral relationship
func (r *UserRepo) CreateReferral(ctx context.Context, tx pgx.Tx, ref *domain.Referral) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO referrals (id, referrer_id, referee_id, total_paid_ltv, total_reward, created_at)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		ref.ID, ref.ReferrerID, ref.RefereeID, ref.TotalPaidLTV, ref.TotalReward, ref.CreatedAt)
	return err
}

// UpdateReferralTotals increments total_paid_ltv and total_reward
func (r *UserRepo) UpdateReferralTotals(ctx context.Context, tx pgx.Tx, referralID uuid.UUID, ltvDelta, rewardDelta int64) error {
	_, err := tx.Exec(ctx, `
		UPDATE referrals SET total_paid_ltv=total_paid_ltv+$1, total_reward=total_reward+$2
		WHERE id=$3`, ltvDelta, rewardDelta, referralID)
	return err
}

// DeleteUserByID used only in tests / admin cleanup
func (r *UserRepo) DeleteUserByID(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM users WHERE id=$1`, userID)
	return err
}

// ─── Referral Reward helpers ──────────────────────────────────────────────────

func (r *UserRepo) CreateReferralReward(ctx context.Context, tx pgx.Tx, rr *domain.ReferralReward) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO referral_rewards (
			id, referral_id, payment_id, referrer_id,
			amount_yad, immediate_yad, deferred_yad,
			status, risk_score, scheduled_at, deferred_at, paid_at, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (payment_id) DO NOTHING`,
		rr.ID, rr.ReferralID, rr.PaymentID, rr.ReferrerID,
		rr.AmountYAD, rr.ImmediateYAD, rr.DeferredYAD,
		rr.Status, rr.RiskScore, rr.ScheduledAt, rr.DeferredAt, rr.PaidAt, rr.CreatedAt,
	)
	return err
}

// GetPendingRewardsDue returns rewards that should be paid out now
func (r *UserRepo) GetPendingRewardsDue(ctx context.Context) ([]*domain.ReferralReward, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, referral_id, payment_id, referrer_id,
		       amount_yad, immediate_yad, deferred_yad,
		       status, risk_score, scheduled_at, deferred_at, paid_at, created_at
		FROM referral_rewards
		WHERE status='pending' AND scheduled_at <= NOW()
		ORDER BY scheduled_at ASC
		LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanReferralRewards(rows)
}

// GetDeferredRewardsDue returns 70% deferred rewards that are now due (30 days)
func (r *UserRepo) GetDeferredRewardsDue(ctx context.Context) ([]*domain.ReferralReward, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, referral_id, payment_id, referrer_id,
		       amount_yad, immediate_yad, deferred_yad,
		       status, risk_score, scheduled_at, deferred_at, paid_at, created_at
		FROM referral_rewards
		WHERE status='immediate' AND deferred_at <= NOW()
		LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanReferralRewards(rows)
}

// UpdateRewardStatus updates reward status
func (r *UserRepo) UpdateRewardStatus(ctx context.Context, tx pgx.Tx, id uuid.UUID, status domain.RewardSplitStatus) error {
	_, err := tx.Exec(ctx, `UPDATE referral_rewards SET status=$1 WHERE id=$2`, status, id)
	return err
}

func scanReferralRewards(rows pgx.Rows) ([]*domain.ReferralReward, error) {
	var rewards []*domain.ReferralReward
	for rows.Next() {
		rr := &domain.ReferralReward{}
		if err := rows.Scan(
			&rr.ID, &rr.ReferralID, &rr.PaymentID, &rr.ReferrerID,
			&rr.AmountYAD, &rr.ImmediateYAD, &rr.DeferredYAD,
			&rr.Status, &rr.RiskScore, &rr.ScheduledAt, &rr.DeferredAt, &rr.PaidAt, &rr.CreatedAt,
		); err != nil {
			return nil, err
		}
		rewards = append(rewards, rr)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return rewards, nil
}

// UpdateExpiredSubscriptions marks all expired active/trial subscriptions
func (r *UserRepo) UpdateExpiredSubscriptions(ctx context.Context) (int64, error) {
	tag, err := r.db.Exec(ctx, `
		UPDATE subscriptions SET status='expired', updated_at=NOW()
		WHERE status IN ('active','trial') AND expires_at <= NOW()`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// ─── Subscription helpers ─────────────────────────────────────────────────────

func (r *UserRepo) CreateSubscription(ctx context.Context, tx pgx.Tx, s *domain.Subscription) error {
	const q = `
		INSERT INTO subscriptions (id, user_id, plan, status, starts_at, expires_at, remna_sub_uuid, paid_kopecks, payment_id, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`
	args := []any{s.ID, s.UserID, s.Plan, s.Status, s.StartsAt, s.ExpiresAt, s.RemnaSubUUID, s.PaidKopecks, s.PaymentID, s.CreatedAt, s.UpdatedAt}
	if tx != nil {
		_, err := tx.Exec(ctx, q, args...)
		return err
	}
	_, err := r.db.Exec(ctx, q, args...)
	return err
}

func (r *UserRepo) GetActiveSubscription(ctx context.Context, userID uuid.UUID) (*domain.Subscription, error) {
	s := &domain.Subscription{}
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, plan, status, starts_at, expires_at, remna_sub_uuid, paid_kopecks, payment_id, created_at, updated_at
		FROM subscriptions WHERE user_id=$1 AND status IN ('active','trial') AND expires_at > NOW()
		ORDER BY expires_at DESC LIMIT 1`, userID).Scan(
		&s.ID, &s.UserID, &s.Plan, &s.Status, &s.StartsAt, &s.ExpiresAt,
		&s.RemnaSubUUID, &s.PaidKopecks, &s.PaymentID, &s.CreatedAt, &s.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return s, err
}

func (r *UserRepo) GetSubscriptionByID(ctx context.Context, id uuid.UUID) (*domain.Subscription, error) {
	s := &domain.Subscription{}
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, plan, status, starts_at, expires_at, remna_sub_uuid, paid_kopecks, payment_id, created_at, updated_at
		FROM subscriptions WHERE id=$1`, id).Scan(
		&s.ID, &s.UserID, &s.Plan, &s.Status, &s.StartsAt, &s.ExpiresAt,
		&s.RemnaSubUUID, &s.PaidKopecks, &s.PaymentID, &s.CreatedAt, &s.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return s, err
}

func (r *UserRepo) ListSubscriptions(ctx context.Context, userID uuid.UUID) ([]*domain.Subscription, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, plan, status, starts_at, expires_at, remna_sub_uuid, paid_kopecks, payment_id, created_at, updated_at
		FROM subscriptions WHERE user_id=$1 ORDER BY created_at DESC`, userID)
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

// GetSubscriptionsExpiringIn returns active subscriptions expiring within the given duration.
func (r *UserRepo) GetSubscriptionsExpiringIn(ctx context.Context, within time.Duration) ([]*domain.Subscription, error) {
	deadline := time.Now().Add(within)
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, plan, status, starts_at, expires_at, remna_sub_uuid, paid_kopecks, payment_id, created_at, updated_at
		FROM subscriptions
		WHERE status = 'active' AND expires_at > NOW() AND expires_at <= $1
		ORDER BY expires_at ASC`, deadline)
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

func (r *UserRepo) ExtendSubscription(ctx context.Context, tx pgx.Tx, subID uuid.UUID, newExpiry time.Time) error {
	if tx != nil {
		_, err := tx.Exec(ctx,
			`UPDATE subscriptions SET expires_at=$1, status='active', updated_at=NOW() WHERE id=$2`,
			newExpiry, subID)
		return err
	}
	_, err := r.db.Exec(ctx,
		`UPDATE subscriptions SET expires_at=$1, status='active', updated_at=NOW() WHERE id=$2`,
		newExpiry, subID)
	return err
}

func (r *UserRepo) UpdateSubscriptionRemna(ctx context.Context, subID uuid.UUID, remnaUUID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE subscriptions SET remna_sub_uuid=$1, updated_at=NOW() WHERE id=$2`,
		remnaUUID, subID)
	return err
}

// ─── Payment helpers ──────────────────────────────────────────────────────────

func (r *UserRepo) CreatePayment(ctx context.Context, p *domain.Payment) error {
	h := sha256.Sum256([]byte(p.ID.String() + string(p.Status)))
	idempotency := hex.EncodeToString(h[:])
	_, err := r.db.Exec(ctx, `
		INSERT INTO payments (id, user_id, amount_kopecks, currency, status, plan,
		                      payment_method, platega_payload, redirect_url, idempotency,
		                      expires_at, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		p.ID, p.UserID, p.AmountKopecks, p.Currency, p.Status, p.Plan,
		p.PaymentMethod, p.PlategaPayload, p.RedirectURL, idempotency,
		p.ExpiresAt, p.CreatedAt, p.UpdatedAt,
	)
	return err
}

func (r *UserRepo) GetPaymentByID(ctx context.Context, id uuid.UUID) (*domain.Payment, error) {
	p := &domain.Payment{}
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, amount_kopecks, currency, status, plan,
		       payment_method, platega_payload, redirect_url, webhook_received_at, idempotency,
		       expires_at, created_at, updated_at
		FROM payments WHERE id=$1`, id).Scan(
		&p.ID, &p.UserID, &p.AmountKopecks, &p.Currency, &p.Status, &p.Plan,
		&p.PaymentMethod, &p.PlategaPayload, &p.RedirectURL, &p.WebhookReceivedAt, &p.Idempotency,
		&p.ExpiresAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return p, err
}

// GetUserPaymentByID returns a payment only if it belongs to the given user.
func (r *UserRepo) GetUserPaymentByID(ctx context.Context, userID, paymentID uuid.UUID) (*domain.Payment, error) {
	p := &domain.Payment{}
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, amount_kopecks, currency, status, plan,
		       payment_method, platega_payload, redirect_url, webhook_received_at, idempotency,
		       expires_at, created_at, updated_at
		FROM payments WHERE id=$1 AND user_id=$2`, paymentID, userID).Scan(
		&p.ID, &p.UserID, &p.AmountKopecks, &p.Currency, &p.Status, &p.Plan,
		&p.PaymentMethod, &p.PlategaPayload, &p.RedirectURL, &p.WebhookReceivedAt, &p.Idempotency,
		&p.ExpiresAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return p, err
}

// GetPendingPaymentByPlan returns the most recent non-expired PENDING payment for
// a user+plan combination, or nil if none exists. Used to deduplicate payment
// initiations and avoid burning through the rate limit on retries.
func (r *UserRepo) GetPendingPaymentByPlan(ctx context.Context, userID uuid.UUID, plan domain.SubscriptionPlan) (*domain.Payment, error) {
	p := &domain.Payment{}
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, amount_kopecks, currency, status, plan,
		       payment_method, platega_payload, redirect_url, webhook_received_at, idempotency,
		       expires_at, created_at, updated_at
		FROM payments
		WHERE user_id=$1 AND plan=$2 AND status='PENDING'
		  AND (expires_at IS NULL OR expires_at > NOW())
		ORDER BY created_at DESC LIMIT 1`, userID, plan).Scan(
		&p.ID, &p.UserID, &p.AmountKopecks, &p.Currency, &p.Status, &p.Plan,
		&p.PaymentMethod, &p.PlategaPayload, &p.RedirectURL, &p.WebhookReceivedAt, &p.Idempotency,
		&p.ExpiresAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return p, err
}

// GetPendingPayments returns non-expired PENDING payments for a user.
func (r *UserRepo) GetPendingPayments(ctx context.Context, userID uuid.UUID) ([]*domain.Payment, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, amount_kopecks, currency, status, plan,
		       payment_method, platega_payload, redirect_url, webhook_received_at, idempotency,
		       expires_at, created_at, updated_at
		FROM payments
		WHERE user_id=$1 AND status='PENDING'
		  AND (expires_at IS NULL OR expires_at > NOW())
		ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPayments(rows)
}

// MarkExpiredPayments sets status=EXPIRED for all PENDING payments past their expires_at.
func (r *UserRepo) MarkExpiredPayments(ctx context.Context) (int64, error) {
	tag, err := r.db.Exec(ctx, `
		UPDATE payments SET status='EXPIRED', updated_at=NOW()
		WHERE status='PENDING' AND expires_at IS NOT NULL AND expires_at <= NOW()`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// GetStalePendingPayments returns PENDING payments that are about to expire (within 2 minutes)
// or are already past expires_at — used by worker to sync with Platega before marking expired.
func (r *UserRepo) GetStalePendingPayments(ctx context.Context) ([]*domain.Payment, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, amount_kopecks, currency, status, plan,
		       payment_method, platega_payload, redirect_url, webhook_received_at, idempotency,
		       expires_at, created_at, updated_at
		FROM payments
		WHERE status='PENDING'
		  AND expires_at IS NOT NULL AND expires_at <= NOW() + INTERVAL '2 minutes'
		ORDER BY expires_at ASC
		LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPayments(rows)
}

func scanPayments(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
}) ([]*domain.Payment, error) {
	var payments []*domain.Payment
	for rows.Next() {
		p := &domain.Payment{}
		if err := rows.Scan(
			&p.ID, &p.UserID, &p.AmountKopecks, &p.Currency, &p.Status, &p.Plan,
			&p.PaymentMethod, &p.PlategaPayload, &p.RedirectURL, &p.WebhookReceivedAt, &p.Idempotency,
			&p.ExpiresAt, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		payments = append(payments, p)
	}
	return payments, rows.Err()
}

func (r *UserRepo) UpdatePaymentStatus(ctx context.Context, tx pgx.Tx, id uuid.UUID, status domain.PaymentStatus) error {
	sql := `UPDATE payments SET status=$1, webhook_received_at=NOW(), updated_at=NOW() WHERE id=$2`
	if tx != nil {
		_, err := tx.Exec(ctx, sql, status, id)
		return err
	}
	_, err := r.db.Exec(ctx, sql, status, id)
	return err
}

// ─── Webhook event idempotency ────────────────────────────────────────────────

// EnsureWebhookEvent inserts the event and returns true if it was new (not duplicate)
func (r *UserRepo) EnsureWebhookEvent(ctx context.Context, source, externalID, eventType string, payload []byte) (bool, error) {
	tag, err := r.db.Exec(ctx, `
		INSERT INTO webhook_events (id, source, external_id, event_type, payload, created_at)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, NOW())
		ON CONFLICT (source, external_id, event_type) DO NOTHING`,
		source, externalID, eventType, payload,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (r *UserRepo) MarkWebhookProcessed(ctx context.Context, source, externalID, eventType string, errMsg string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE webhook_events SET processed=TRUE, processed_at=NOW(), error=$4
		WHERE source=$1 AND external_id=$2 AND event_type=$3`,
		source, externalID, eventType, errMsg,
	)
	return err
}

// ─── Promo codes ──────────────────────────────────────────────────────────────

func (r *UserRepo) GetPromoByCode(ctx context.Context, code string) (*domain.PromoCode, error) {
	p := &domain.PromoCode{}
	err := r.db.QueryRow(ctx, `
		SELECT id, code, promo_type, yad_amount, discount_percent, max_uses, used_count, expires_at, created_by_id, created_at
		FROM promocodes WHERE code=$1`, code).Scan(
		&p.ID, &p.Code, &p.PromoType, &p.YADAmount, &p.DiscountPercent, &p.MaxUses, &p.UsedCount, &p.ExpiresAt, &p.CreatedByID, &p.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return p, err
}

func (r *UserRepo) UsePromoCode(ctx context.Context, tx pgx.Tx, promoID, userID uuid.UUID) error {
	// Record the use atomically with counter increment
	_, err := tx.Exec(ctx, `
		UPDATE promocodes SET used_count=used_count+1 WHERE id=$1 AND used_count < max_uses`,
		promoID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO promocode_uses (id, promo_code_id, user_id, used_at)
		VALUES (gen_random_uuid(), $1, $2, NOW())`,
		promoID, userID,
	)
	return err
}

func (r *UserRepo) HasUserUsedPromo(ctx context.Context, promoID, userID uuid.UUID) (bool, error) {
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM promocode_uses WHERE promo_code_id=$1 AND user_id=$2`,
		promoID, userID).Scan(&count)
	return count > 0, err
}

func (r *UserRepo) CreatePromoCode(ctx context.Context, p *domain.PromoCode) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO promocodes (id, code, promo_type, yad_amount, discount_percent, max_uses, used_count, expires_at, created_by_id, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		p.ID, p.Code, p.PromoType, p.YADAmount, p.DiscountPercent, p.MaxUses, p.UsedCount, p.ExpiresAt, p.CreatedByID, p.CreatedAt,
	)
	return err
}

func (r *UserRepo) ListPromoCodes(ctx context.Context) ([]*domain.PromoCode, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, code, promo_type, yad_amount, discount_percent, max_uses, used_count, expires_at, created_by_id, created_at
		FROM promocodes ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*domain.PromoCode
	for rows.Next() {
		p := &domain.PromoCode{}
		if err := rows.Scan(&p.ID, &p.Code, &p.PromoType, &p.YADAmount, &p.DiscountPercent, &p.MaxUses, &p.UsedCount,
			&p.ExpiresAt, &p.CreatedByID, &p.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

// SetActiveDiscount stores a pending discount promo on the user record.
func (r *UserRepo) SetActiveDiscount(ctx context.Context, userID uuid.UUID, code string, percent int) error {
	_, err := r.db.Exec(ctx,
		`UPDATE users SET active_discount_code=$1, active_discount_percent=$2, updated_at=NOW() WHERE id=$3`,
		code, percent, userID)
	return err
}

// GetActiveDiscount returns the user's pending discount code and percent.
func (r *UserRepo) GetActiveDiscount(ctx context.Context, userID uuid.UUID) (string, int, error) {
	var code *string
	var percent int
	err := r.db.QueryRow(ctx,
		`SELECT active_discount_code, active_discount_percent FROM users WHERE id=$1`, userID).
		Scan(&code, &percent)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", 0, nil
	}
	if err != nil {
		return "", 0, err
	}
	if code == nil {
		return "", 0, nil
	}
	return *code, percent, nil
}

// ClearActiveDiscount removes the user's pending discount after it has been applied.
func (r *UserRepo) ClearActiveDiscount(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE users SET active_discount_code=NULL, active_discount_percent=0, updated_at=NOW() WHERE id=$1`,
		userID)
	return err
}

// ─── Tickets ──────────────────────────────────────────────────────────────────

func (r *UserRepo) CreateTicket(ctx context.Context, t *domain.Ticket) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO tickets (id, user_id, subject, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		t.ID, t.UserID, t.Subject, t.Status, t.CreatedAt, t.UpdatedAt,
	)
	return err
}

func (r *UserRepo) GetTicketByID(ctx context.Context, id uuid.UUID) (*domain.Ticket, error) {
	t := &domain.Ticket{}
	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, subject, status, created_at, updated_at FROM tickets WHERE id=$1`, id).Scan(
		&t.ID, &t.UserID, &t.Subject, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return t, err
}

func (r *UserRepo) ListTickets(ctx context.Context, userID *uuid.UUID, status string, limit, offset int) ([]*domain.Ticket, error) {
	query := `SELECT id, user_id, subject, status, created_at, updated_at FROM tickets WHERE 1=1`
	args := []interface{}{}
	n := 1
	if userID != nil {
		query += fmt.Sprintf(" AND user_id=$%d", n)
		args = append(args, *userID)
		n++
	}
	if status != "" {
		query += fmt.Sprintf(" AND status=$%d", n)
		args = append(args, status)
		n++
	}
	query += fmt.Sprintf(" ORDER BY updated_at DESC LIMIT $%d OFFSET $%d", n, n+1)
	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tickets []*domain.Ticket
	for rows.Next() {
		t := &domain.Ticket{}
		if err := rows.Scan(&t.ID, &t.UserID, &t.Subject, &t.Status, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tickets = append(tickets, t)
	}
	return tickets, rows.Err()
}

func (r *UserRepo) AddTicketMessage(ctx context.Context, m *domain.TicketMessage) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO ticket_messages (id, ticket_id, sender_id, is_admin, body, created_at)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		m.ID, m.TicketID, m.SenderID, m.IsAdmin, m.Body, m.CreatedAt,
	)
	return err
}

func (r *UserRepo) GetTicketMessages(ctx context.Context, ticketID uuid.UUID) ([]*domain.TicketMessage, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, ticket_id, sender_id, is_admin, body, created_at FROM ticket_messages WHERE ticket_id=$1 ORDER BY created_at ASC`,
		ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var msgs []*domain.TicketMessage
	for rows.Next() {
		m := &domain.TicketMessage{}
		if err := rows.Scan(&m.ID, &m.TicketID, &m.SenderID, &m.IsAdmin, &m.Body, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

func (r *UserRepo) UpdateTicketStatus(ctx context.Context, ticketID uuid.UUID, status domain.TicketStatus) error {
	_, err := r.db.Exec(ctx,
		`UPDATE tickets SET status=$1, updated_at=NOW() WHERE id=$2`, status, ticketID)
	return err
}

// ─── Shop ─────────────────────────────────────────────────────────────────────

func (r *UserRepo) ListShopItems(ctx context.Context) ([]*domain.ShopItem, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, name, description, price_yad, stock, is_active, created_at FROM shop_items WHERE is_active=TRUE ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []*domain.ShopItem
	for rows.Next() {
		item := &domain.ShopItem{}
		if err := rows.Scan(&item.ID, &item.Name, &item.Description, &item.PriceYAD, &item.Stock, &item.IsActive, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *UserRepo) GetShopItemByID(ctx context.Context, id uuid.UUID) (*domain.ShopItem, error) {
	item := &domain.ShopItem{}
	err := r.db.QueryRow(ctx,
		`SELECT id, name, description, price_yad, stock, is_active, created_at FROM shop_items WHERE id=$1`,
		id).Scan(&item.ID, &item.Name, &item.Description, &item.PriceYAD, &item.Stock, &item.IsActive, &item.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return item, err
}

func (r *UserRepo) BuyShopItem(ctx context.Context, tx pgx.Tx, order *domain.ShopOrder) error {
	// Decrement stock if not unlimited
	_, err := tx.Exec(ctx, `
		UPDATE shop_items SET stock = CASE WHEN stock > 0 THEN stock - $1 ELSE stock END
		WHERE id=$2 AND is_active=TRUE AND (stock >= $1 OR stock = -1)`,
		order.Quantity, order.ItemID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO shop_orders (id, user_id, item_id, quantity, total_yad, created_at)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		order.ID, order.UserID, order.ItemID, order.Quantity, order.TotalYAD, order.CreatedAt,
	)
	return err
}

// ─── Risk events ──────────────────────────────────────────────────────────────

func (r *UserRepo) CreateRiskEvent(ctx context.Context, userID *uuid.UUID, eventType, ip, fp string, delta int, details []byte) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO risk_events (id, user_id, event_type, ip, fingerprint, score_delta, details, created_at)
		VALUES (gen_random_uuid(),$1,$2,$3::inet,$4,$5,$6,NOW())`,
		userID, eventType, ip, fp, delta, details,
	)
	return err
}

// ─── Analytics ────────────────────────────────────────────────────────────────

func (r *UserRepo) GetAnalytics(ctx context.Context) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	var totalUsers, bannedUsers, totalSubs, activeSubs int64
	var totalRevenue, totalYADCirculation int64
	var pendingRewards int64
	var openTickets, highRiskUsers int64

	_ = r.db.QueryRow(ctx, `SELECT COUNT(*), COUNT(*) FILTER (WHERE is_banned) FROM users`).
		Scan(&totalUsers, &bannedUsers)
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*), COUNT(*) FILTER (WHERE status='active') FROM subscriptions`).
		Scan(&totalSubs, &activeSubs)
	_ = r.db.QueryRow(ctx, `SELECT COALESCE(SUM(amount_kopecks),0) FROM payments WHERE status='CONFIRMED'`).
		Scan(&totalRevenue)
	_ = r.db.QueryRow(ctx, `SELECT COALESCE(SUM(yad_balance),0) FROM users`).
		Scan(&totalYADCirculation)
	_ = r.db.QueryRow(ctx, `SELECT COALESCE(SUM(deferred_yad),0) FROM referral_rewards WHERE status IN ('pending','immediate')`).
		Scan(&pendingRewards)
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM tickets WHERE status='open'`).
		Scan(&openTickets)
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE risk_score >= 70`).
		Scan(&highRiskUsers)

	result["total_users"] = totalUsers
	result["banned_users"] = bannedUsers
	result["total_subscriptions"] = totalSubs
	result["active_subscriptions"] = activeSubs
	result["total_revenue_kopecks"] = totalRevenue
	result["yad_in_circulation"] = totalYADCirculation
	result["pending_rewards"] = pendingRewards
	result["open_tickets"] = openTickets
	result["high_risk_users"] = highRiskUsers
	return result, nil
}

// ─── Account Merge ────────────────────────────────────────────────────────────

// MergeResult summarises what was transferred from the source (bot) account.
type MergeResult struct {
	TransferredYAD      int64
	TransferredSubs     int64
	TransferredPayments int64
}

// MergeAccounts merges the src (bot) account into the dst (website) account inside a
// serializable transaction. All child records are re-assigned, balances are added,
// and the src user is deleted at the end.
//
// Safe to call when only one side has referral/subscriptions; handles all FK constraints.
func (r *UserRepo) MergeAccounts(ctx context.Context, dstID, srcID uuid.UUID) (*MergeResult, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return nil, fmt.Errorf("begin merge tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// ── Snapshot what we are transferring ──────────────────────────────────
	res := &MergeResult{}
	if err := tx.QueryRow(ctx, `SELECT yad_balance FROM users WHERE id=$1`, srcID).Scan(&res.TransferredYAD); err != nil {
		return nil, fmt.Errorf("snapshot yad: %w", err)
	}
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM subscriptions WHERE user_id=$1`, srcID).Scan(&res.TransferredSubs); err != nil {
		return nil, fmt.Errorf("snapshot subs: %w", err)
	}
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM payments WHERE user_id=$1`, srcID).Scan(&res.TransferredPayments); err != nil {
		return nil, fmt.Errorf("snapshot payments: %w", err)
	}

	// Detach src.telegram_id before assigning it to dst to avoid UNIQUE conflict
	// on users(telegram_id) while both rows still exist in this transaction.
	var srcTelegramID *int64
	if err := tx.QueryRow(ctx, `SELECT telegram_id FROM users WHERE id=$1`, srcID).Scan(&srcTelegramID); err != nil {
		return nil, fmt.Errorf("snapshot src telegram_id: %w", err)
	}
	if srcTelegramID != nil {
		if _, err := tx.Exec(ctx, `UPDATE users SET telegram_id=NULL, updated_at=NOW() WHERE id=$1`, srcID); err != nil {
			return nil, fmt.Errorf("detach src telegram_id: %w", err)
		}
	}

	// ── 1. Merge scalar fields on dst ──────────────────────────────────────
	_, err = tx.Exec(ctx, `
		UPDATE users SET
			yad_balance           = yad_balance + (SELECT yad_balance FROM users WHERE id=$2),
			ltv                   = ltv         + (SELECT ltv         FROM users WHERE id=$2),
			trial_used            = trial_used  OR (SELECT trial_used FROM users WHERE id=$2),
			telegram_id           = COALESCE($3, telegram_id),
			remna_user_uuid       = COALESCE(remna_user_uuid,       (SELECT remna_user_uuid       FROM users WHERE id=$2)),
			active_discount_code  = COALESCE(active_discount_code,  (SELECT active_discount_code  FROM users WHERE id=$2)),
			active_discount_percent = CASE
				WHEN active_discount_percent > 0 THEN active_discount_percent
				ELSE (SELECT active_discount_percent FROM users WHERE id=$2)
			END,
			updated_at = NOW()
		WHERE id=$1`, dstID, srcID, srcTelegramID)
	if err != nil {
		return nil, fmt.Errorf("merge user fields: %w", err)
	}

	// ── 1b. Point all src subscriptions to dst's remna_user_uuid so that
	//        VPN renewals use the correct (surviving) Remnawave account.
	_, err = tx.Exec(ctx, `
		UPDATE subscriptions
		SET    remna_sub_uuid = (SELECT remna_user_uuid FROM users WHERE id=$1),
		       updated_at     = NOW()
		WHERE  user_id = $2
		  AND  (SELECT remna_user_uuid FROM users WHERE id=$1) IS NOT NULL`,
		dstID, srcID)
	if err != nil {
		return nil, fmt.Errorf("repoint src subs remna_uuid: %w", err)
	}

	// ── 2. Re-assign simple user_id / sender_id FKs ────────────────────────
	for _, q := range []string{
		`UPDATE subscriptions    SET user_id   = $1 WHERE user_id   = $2`,
		`UPDATE payments         SET user_id   = $1 WHERE user_id   = $2`,
		`UPDATE yad_transactions SET user_id   = $1 WHERE user_id   = $2`,
		`UPDATE tickets          SET user_id   = $1 WHERE user_id   = $2`,
		`UPDATE ticket_messages  SET sender_id = $1 WHERE sender_id = $2`,
		`UPDATE shop_orders      SET user_id   = $1 WHERE user_id   = $2`,
		`UPDATE referral_rewards SET referrer_id = $1 WHERE referrer_id = $2`,
		`UPDATE referrals        SET referrer_id = $1 WHERE referrer_id = $2`,
	} {
		if _, err := tx.Exec(ctx, q, dstID, srcID); err != nil {
			return nil, fmt.Errorf("re-assign (%s): %w", q[:40], err)
		}
	}

	// ── 3. Deduplicate and re-assign promocode_uses ────────────────────────
	if _, err := tx.Exec(ctx, `
		DELETE FROM promocode_uses
		WHERE user_id=$2
		  AND promo_code_id IN (SELECT promo_code_id FROM promocode_uses WHERE user_id=$1)`,
		dstID, srcID); err != nil {
		return nil, fmt.Errorf("dedup promocode_uses: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE promocode_uses SET user_id=$1 WHERE user_id=$2`, dstID, srcID); err != nil {
		return nil, fmt.Errorf("merge promocode_uses: %w", err)
	}

	// ── 4. Handle the referral where src was the REFEREE ──────────────────
	// (rewards where src.referrer gets paid were already moved in step 2)
	var srcReferralID uuid.UUID
	scanErr := tx.QueryRow(ctx, `SELECT id FROM referrals WHERE referee_id=$1`, srcID).Scan(&srcReferralID)
	if scanErr == nil {
		var dstHasReferral bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM referrals WHERE referee_id=$1)`, dstID).Scan(&dstHasReferral); err != nil {
			return nil, fmt.Errorf("check dst referral: %w", err)
		}
		if !dstHasReferral {
			// Transfer: dst becomes the referee
			if _, err := tx.Exec(ctx, `UPDATE referrals SET referee_id=$1 WHERE id=$2`, dstID, srcReferralID); err != nil {
				return nil, fmt.Errorf("transfer referee: %w", err)
			}
		} else {
			// dst already has a referral — delete src's referral chain
			if _, err := tx.Exec(ctx, `DELETE FROM referral_rewards WHERE referral_id=$1`, srcReferralID); err != nil {
				return nil, fmt.Errorf("delete src referral_rewards: %w", err)
			}
			if _, err := tx.Exec(ctx, `DELETE FROM referrals WHERE id=$1`, srcReferralID); err != nil {
				return nil, fmt.Errorf("delete src referral: %w", err)
			}
		}
	} else if !errors.Is(scanErr, pgx.ErrNoRows) {
		return nil, fmt.Errorf("fetch src referral: %w", scanErr)
	}

	// ── 5. Delete src user (admin_audit_logs has ON DELETE SET NULL) ───────
	if _, err := tx.Exec(ctx, `DELETE FROM users WHERE id=$1`, srcID); err != nil {
		return nil, fmt.Errorf("delete src user: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit merge: %w", err)
	}
	return res, nil
}
