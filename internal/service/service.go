// Package service contains all business logic for the VPN platform.
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/vpnplatform/internal/anticheat"
	"github.com/vpnplatform/internal/domain"
	"github.com/vpnplatform/internal/integration/platega"
	"github.com/vpnplatform/internal/integration/remnawave"
	"github.com/vpnplatform/internal/repository/postgres"
	"github.com/vpnplatform/internal/worker"
	"github.com/vpnplatform/pkg/password"
)

// ─── Auth Service ─────────────────────────────────────────────────────────────

type AuthService struct {
	repo       *postgres.UserRepo
	anti       *anticheat.Engine
	log        *zap.Logger
	adminLogin string // login that is auto-granted is_admin
}

func NewAuthService(repo *postgres.UserRepo, anti *anticheat.Engine, log *zap.Logger, adminLogin string) *AuthService {
	return &AuthService{repo: repo, anti: anti, log: log, adminLogin: adminLogin}
}

type RegisterInput struct {
	Username          string
	Password          string
	ReferralCode      string // optional
	DeviceFingerprint string
	IP                string
}

func (s *AuthService) Register(ctx context.Context, input RegisterInput) (*domain.User, error) {
	username := strings.TrimSpace(input.Username)
	if len(username) < 3 || len(username) > 64 {
		return nil, errors.New("username must be 3-64 characters")
	}
	if len(input.Password) < 8 {
		return nil, errors.New("password must be at least 8 characters")
	}

	// Rate limit registration by IP
	if err := s.anti.CheckIPRateLimit(ctx, input.IP, "register", 5, time.Hour); err != nil {
		return nil, err
	}

	existing, err := s.repo.GetByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, errors.New("username already registered")
	}

	// Check for IP abuse
	newUserID := uuid.New()
	sameIPCount, _ := s.repo.CountUsersFromIP(ctx, input.IP, newUserID)

	// Fingerprint check: count users with same fingerprint
	// (simplified — full implementation checks device_fingerprint column separately)
	riskDelta := s.anti.ScopeRegistrationRisk(ctx, input.IP, input.DeviceFingerprint, sameIPCount, 0)

	hash, err := password.Hash(input.Password)
	if err != nil {
		return nil, err
	}

	refCode, err := generateReferralCode()
	if err != nil {
		return nil, err
	}

	user := &domain.User{
		ID:                newUserID,
		Username:          &username,
		PasswordHash:      &hash,
		YADBalance:        0,
		ReferralCode:      refCode,
		RiskScore:         riskDelta,
		IsAdmin:           s.adminLogin != "" && strings.EqualFold(username, s.adminLogin),
		DeviceFingerprint: &input.DeviceFingerprint,
		LastKnownIP:       &input.IP,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	// Resolve referrer
	if input.ReferralCode != "" {
		referrer, err := s.repo.GetByReferralCode(ctx, input.ReferralCode)
		if err != nil {
			return nil, err
		}
		if referrer != nil {
			if err := s.anti.CheckSelfReferral(user.ID, referrer.ID); err != nil {
				s.log.Warn("self-referral attempt blocked", zap.String("user_id", user.ID.String()))
				riskDelta += anticheat.DeltaSelfReferral
				user.RiskScore = anticheat.ClampRiskScore(user.RiskScore + anticheat.DeltaSelfReferral)
			} else {
				if err := s.anti.CheckDailyReferralLimit(ctx, referrer.ID); err != nil {
					s.log.Warn("referral daily limit", zap.String("referrer_id", referrer.ID.String()))
				} else {
					user.ReferrerID = &referrer.ID
				}
			}
		}
	}

	if err := s.repo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	// Create the referral record if a valid referrer was assigned
	if user.ReferrerID != nil {
		tx, err := s.repo.BeginTx(ctx)
		if err == nil {
			ref := &domain.Referral{
				ID:         uuid.New(),
				ReferrerID: *user.ReferrerID,
				RefereeID:  user.ID,
				CreatedAt:  time.Now(),
			}
			_ = s.repo.CreateReferral(ctx, tx, ref)
			_ = tx.Commit(ctx)
		}
	}

	s.log.Info("user registered", zap.String("user_id", user.ID.String()), zap.Int("risk_score", user.RiskScore))
	return user, nil
}

type LoginInput struct {
	Username string
	Password string
	IP       string
}

func (s *AuthService) Login(ctx context.Context, input LoginInput) (*domain.User, error) {
	username := strings.TrimSpace(input.Username)

	// Brute-force check before querying DB
	if err := s.anti.CheckLoginRateLimit(ctx, input.IP+":"+username); err != nil {
		return nil, err
	}

	user, err := s.repo.GetByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if user == nil || user.PasswordHash == nil {
		return nil, errors.New("invalid credentials")
	}
	if !password.Verify(*user.PasswordHash, input.Password) {
		return nil, errors.New("invalid credentials")
	}
	if user.IsBanned {
		return nil, errors.New("account is banned")
	}

	s.anti.ResetLoginAttempts(ctx, input.IP+":"+username)

	// Auto-promote to admin if this login is configured as admin and isn't yet
	if s.adminLogin != "" && strings.EqualFold(username, s.adminLogin) && !user.IsAdmin {
		if err := s.repo.SetAdmin(ctx, user.ID, true); err != nil {
			s.log.Warn("failed to auto-promote admin", zap.Error(err))
		} else {
			user.IsAdmin = true
		}
	}

	return user, nil
}

// ─── Subscription Service ─────────────────────────────────────────────────────

type SubscriptionService struct {
	repo    *postgres.UserRepo
	platega *platega.Client
	remna   *remnawave.Client
	anti    *anticheat.Engine
	rdb     *redis.Client
	log     *zap.Logger
}

func NewSubscriptionService(
	repo *postgres.UserRepo,
	platega *platega.Client,
	remna *remnawave.Client,
	anti *anticheat.Engine,
	rdb *redis.Client,
	log *zap.Logger,
) *SubscriptionService {
	return &SubscriptionService{repo: repo, platega: platega, remna: remna, anti: anti, rdb: rdb, log: log}
}

// InitiatePayment creates a Platega payment session and a pending Payment record.
// Returns the Platega redirect URL.
func (s *SubscriptionService) InitiatePayment(ctx context.Context, userID uuid.UUID, plan domain.SubscriptionPlan, returnURL string) (string, *domain.Payment, error) {
	priceKopecks := domain.PlanPriceKopecks(plan)
	if priceKopecks == 0 {
		return "", nil, errors.New("invalid subscription plan")
	}

	// If the user already has a non-expired PENDING payment for this plan,
	// return it directly — no new charge, no rate-limit consumption.
	existing, err := s.repo.GetPendingPaymentByPlan(ctx, userID, plan)
	if err == nil && existing != nil {
		return existing.RedirectURL, existing, nil
	}

	// Rate-limit: max 5 payment initiations per user per hour
	if err := s.anti.CheckAPIRateLimit(ctx, userID.String(), "payment_init", 5, time.Hour); err != nil {
		return "", nil, err
	}

	user, err := s.repo.GetByID(ctx, userID)
	if err != nil || user == nil {
		return "", nil, errors.New("user not found")
	}

	// Convert kopecks to rubles for Platega
	amountRubles := float64(priceKopecks) / 100.0

	platResp, err := s.platega.CreatePayment(ctx, platega.CreatePaymentRequest{
		PaymentMethod: platega.MethodSBPQR,
		PaymentDetails: platega.PaymentDetails{
			Amount:   amountRubles,
			Currency: "RUB",
		},
		Description: fmt.Sprintf("VPN подписка %s", plan),
		Return:      returnURL,
		Payload:     userID.String(), // our payload to identify user on webhook
	})
	if err != nil {
		return "", nil, fmt.Errorf("initiate payment: %w", err)
	}

	txID, err := uuid.Parse(platResp.TransactionID)
	if err != nil {
		return "", nil, fmt.Errorf("invalid transaction id from Platega: %w", err)
	}

	payment := &domain.Payment{
		ID:             txID,
		UserID:         userID,
		AmountKopecks:  priceKopecks,
		Currency:       "RUB",
		Status:         domain.PaymentStatusPending,
		Plan:           plan,
		PaymentMethod:  platega.MethodSBPQR,
		PlategaPayload: userID.String(),
		RedirectURL:    platResp.Redirect,
		ExpiresAt:      func() *time.Time { t := time.Now().Add(15 * time.Minute); return &t }(),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := s.repo.CreatePayment(ctx, payment); err != nil {
		return "", nil, fmt.Errorf("store payment: %w", err)
	}

	s.log.Info("payment initiated",
		zap.String("user_id", userID.String()),
		zap.String("plan", string(plan)),
		zap.String("tx_id", txID.String()),
	)
	return platResp.Redirect, payment, nil
}

// GetUserSubscriptions returns all subscriptions for a user
func (s *SubscriptionService) GetUserSubscriptions(ctx context.Context, userID uuid.UUID) ([]*domain.Subscription, error) {
	return s.repo.ListSubscriptions(ctx, userID)
}

// GetUserActiveSubscription returns the current active subscription, if any
func (s *SubscriptionService) GetUserActiveSubscription(ctx context.Context, userID uuid.UUID) (*domain.Subscription, error) {
	return s.repo.GetActiveSubscription(ctx, userID)
}

// InitiateRenewal creates a payment for renewing an active or recently expired subscription.
func (s *SubscriptionService) InitiateRenewal(ctx context.Context, userID uuid.UUID, plan domain.SubscriptionPlan, returnURL string) (string, *domain.Payment, error) {
	return s.InitiatePayment(ctx, userID, plan, returnURL)
}

// GetPendingPayments returns non-expired PENDING payments for a user.
func (s *SubscriptionService) GetPendingPayments(ctx context.Context, userID uuid.UUID) ([]*domain.Payment, error) {
	return s.repo.GetPendingPayments(ctx, userID)
}

// GetPaymentByID returns a payment owned by the given user.
func (s *SubscriptionService) GetPaymentByID(ctx context.Context, userID, paymentID uuid.UUID) (*domain.Payment, error) {
	p, err := s.repo.GetUserPaymentByID(ctx, userID, paymentID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, errors.New("payment not found")
	}
	return p, nil
}

// CheckPaymentStatus syncs a pending payment status with Platega and, if
// confirmed, enqueues subscription activation. Returns the updated payment.
func (s *SubscriptionService) CheckPaymentStatus(ctx context.Context, userID, paymentID uuid.UUID) (*domain.Payment, error) {
	payment, err := s.repo.GetUserPaymentByID(ctx, userID, paymentID)
	if err != nil {
		return nil, err
	}
	if payment == nil {
		return nil, errors.New("payment not found")
	}

	// Only PENDING payments need a live check.
	if payment.Status != domain.PaymentStatusPending {
		return payment, nil
	}

	// Expired locally — no point querying Platega.
	if payment.ExpiresAt != nil && payment.ExpiresAt.Before(time.Now()) {
		_ = s.repo.UpdatePaymentStatus(ctx, nil, paymentID, domain.PaymentStatusExpired)
		payment.Status = domain.PaymentStatusExpired
		return payment, nil
	}

	platResp, err := s.platega.GetPaymentStatus(ctx, paymentID.String())
	if err != nil {
		// Log but return the last known DB status — don't fail the request.
		s.log.Warn("platega status check failed", zap.String("payment_id", paymentID.String()), zap.Error(err))
		return payment, nil
	}

	newStatus := domain.PaymentStatus(platResp.Status)
	if newStatus == payment.Status {
		return payment, nil // nothing changed
	}

	// Persist the new status.
	_ = s.repo.UpdatePaymentStatus(ctx, nil, paymentID, newStatus)
	payment.Status = newStatus

	// If confirmed, enqueue the same processing job the webhook would have sent.
	if newStatus == domain.PaymentStatusConfirmed {
		amountKopecks := int64(float64(payment.AmountKopecks)) // already in kopecks
		job := worker.PaymentProcessJob{
			TransactionID: paymentID.String(),
			UserID:        userID.String(),
			AmountKopecks: amountKopecks,
			Plan:          string(payment.Plan),
			Status:        string(newStatus),
		}
		if enqErr := worker.Enqueue(ctx, s.rdb, worker.QueuePaymentProcess, job); enqErr != nil {
			s.log.Error("enqueue payment process after manual check", zap.Error(enqErr))
		}
	}

	return payment, nil
}

// ─── YAD / Economy Service ────────────────────────────────────────────────────

type EconomyService struct {
	repo *postgres.UserRepo
	anti *anticheat.Engine
	log  *zap.Logger
}

func NewEconomyService(repo *postgres.UserRepo, anti *anticheat.Engine, log *zap.Logger) *EconomyService {
	return &EconomyService{repo: repo, anti: anti, log: log}
}

// CreditYAD safely credits YAD to a user, enforcing daily limits.
func (s *EconomyService) CreditYAD(ctx context.Context, userID uuid.UUID, amount int64, txType domain.YADTxType, refID *uuid.UUID, note string) error {
	if amount <= 0 {
		return errors.New("YAD amount must be positive")
	}

	// Daily cap check
	if err := s.anti.CheckAndAddDailyYADCredit(ctx, userID, amount); err != nil {
		return err
	}

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := s.repo.AdjustYADBalance(ctx, tx, userID, amount, txType, refID, note); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// DebitYAD deducts YAD from a user (for shop purchases).
func (s *EconomyService) DebitYAD(ctx context.Context, userID uuid.UUID, amount int64, txType domain.YADTxType, refID *uuid.UUID, note string) error {
	if amount <= 0 {
		return errors.New("debit amount must be positive")
	}
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := s.repo.AdjustYADBalance(ctx, tx, userID, -amount, txType, refID, note); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// UsePromoCode validates and applies a promo code for a user.
func (s *EconomyService) UsePromoCode(ctx context.Context, userID uuid.UUID, code string) (*domain.PromoCode, error) {
	promo, err := s.repo.GetPromoByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if promo == nil {
		return nil, errors.New("promo code not found")
	}
	if promo.ExpiresAt != nil && promo.ExpiresAt.Before(time.Now()) {
		return nil, errors.New("promo code expired")
	}
	if promo.UsedCount >= promo.MaxUses {
		return nil, errors.New("promo code exhausted")
	}

	used, err := s.repo.HasUserUsedPromo(ctx, promo.ID, userID)
	if err != nil {
		return nil, err
	}
	if used {
		return nil, errors.New("promo code already used")
	}

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if err := s.repo.UsePromoCode(ctx, tx, promo.ID, userID); err != nil {
		return nil, err
	}
	refID := promo.ID
	if err := s.repo.AdjustYADBalance(ctx, tx, userID, promo.YADAmount, domain.YADTxPromo, &refID, "Promo: "+code); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	s.log.Info("promo code applied",
		zap.String("user_id", userID.String()),
		zap.String("code", code),
		zap.Int64("yad", promo.YADAmount),
	)
	return promo, nil
}

// BuyShopItem purchases a shop item deducting YAD from the user's balance.
func (s *EconomyService) BuyShopItem(ctx context.Context, userID, itemID uuid.UUID, quantity int) (*domain.ShopOrder, error) {
	if quantity <= 0 {
		return nil, errors.New("quantity must be positive")
	}

	item, err := s.repo.GetShopItemByID(ctx, itemID)
	if err != nil {
		return nil, err
	}
	if item == nil || !item.IsActive {
		return nil, errors.New("item not found")
	}
	if item.Stock != -1 && item.Stock < quantity {
		return nil, errors.New("insufficient stock")
	}

	totalYAD := item.PriceYAD * int64(quantity)

	user, err := s.repo.GetByID(ctx, userID)
	if err != nil || user == nil {
		return nil, errors.New("user not found")
	}
	if user.YADBalance < totalYAD {
		return nil, errors.New("insufficient YAD balance")
	}

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	order := &domain.ShopOrder{
		ID:        uuid.New(),
		UserID:    userID,
		ItemID:    itemID,
		Quantity:  quantity,
		TotalYAD:  totalYAD,
		CreatedAt: time.Now(),
	}

	if err := s.repo.BuyShopItem(ctx, tx, order); err != nil {
		return nil, fmt.Errorf("buy item: %w", err)
	}

	refID := order.ID
	if err := s.repo.AdjustYADBalance(ctx, tx, userID, -totalYAD, domain.YADTxSpent, &refID, "Shop: "+item.Name); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return order, nil
}

// ─── Trial Service ────────────────────────────────────────────────────────────

type TrialService struct {
	repo  *postgres.UserRepo
	remna *remnawave.Client
	log   *zap.Logger
}

func NewTrialService(repo *postgres.UserRepo, remna *remnawave.Client, log *zap.Logger) *TrialService {
	return &TrialService{repo: repo, remna: remna, log: log}
}

// ActivateTrial grants a free 3-day trial to a new user (once per account).
func (s *TrialService) ActivateTrial(ctx context.Context, userID uuid.UUID) (*domain.Subscription, error) {
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil || user == nil {
		return nil, errors.New("user not found")
	}
	if user.TrialUsed {
		return nil, errors.New("trial already used")
	}

	now := time.Now()
	expiry := now.Add(3 * 24 * time.Hour)

	// Create Remnawave user if not yet created
	remnaUUID := ""
	if user.RemnaUserUUID == nil || *user.RemnaUserUUID == "" {
		remnaUser, err := s.remna.CreateUser(ctx, userID.String(), expiry)
		if err != nil {
			return nil, fmt.Errorf("create remnawave user: %w", err)
		}
		remnaUUID = remnaUser.UUID
		_ = s.repo.UpdateRemnaUUID(ctx, userID, remnaUUID)
	} else {
		remnaUUID = *user.RemnaUserUUID
		_ = s.remna.UpdateExpiry(ctx, remnaUUID, expiry)
	}

	sub := &domain.Subscription{
		ID:           uuid.New(),
		UserID:       userID,
		Plan:         domain.PlanWeek,
		Status:       domain.SubStatusTrial,
		StartsAt:     now,
		ExpiresAt:    expiry,
		RemnaSubUUID: &remnaUUID,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if err := s.repo.CreateSubscription(ctx, tx, sub); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	_ = s.repo.SetTrialUsed(ctx, userID)

	s.log.Info("trial activated", zap.String("user_id", userID.String()))
	return sub, nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func generateReferralCode() (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(b)), nil
}
