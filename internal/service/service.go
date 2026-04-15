// Package service contains all business logic for the VPN platform.
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
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
	rdb        *redis.Client
	log        *zap.Logger
	adminLogin string // login that is auto-granted is_admin
}

func NewAuthService(repo *postgres.UserRepo, anti *anticheat.Engine, rdb *redis.Client, log *zap.Logger, adminLogin string) *AuthService {
	// Normalise once at construction so comparisons are always exact (C-5).
	return &AuthService{repo: repo, anti: anti, rdb: rdb, log: log, adminLogin: strings.ToLower(strings.TrimSpace(adminLogin))}
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
		return nil, errors.New("имя пользователя должно содержать от 3 до 64 символов")
	}
	if len(input.Password) < 8 {
		return nil, errors.New("пароль должен содержать не менее 8 символов")
	}

	// Rate limit registration by IP (max 3 per hour).
	// If IP is unknown/invalid (e.g. Telegram bot registrations), skip IP-based limiting.
	ipOK := net.ParseIP(strings.TrimSpace(input.IP)) != nil
	if ipOK {
		if err := s.anti.CheckIPRateLimit(ctx, input.IP, "register", 3, time.Hour); err != nil {
			return nil, err
		}
	}

	existing, err := s.repo.GetByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, errors.New("это имя пользователя уже занято")
	}

	// Check for IP and device-fingerprint abuse (H-5).
	newUserID := uuid.New()
	sameIPCount := 0
	if ipOK {
		sameIPCount, _ = s.repo.CountUsersFromIP(ctx, input.IP, newUserID)
	}
	sameFPCount, _ := s.repo.CountUsersFromFingerprint(ctx, input.DeviceFingerprint, newUserID)

	// Hard block: max 3 accounts per IP, max 2 per device fingerprint
	if sameIPCount >= 3 {
		s.log.Warn("registration blocked: too many accounts from IP",
			zap.String("ip", input.IP), zap.Int("count", sameIPCount))
		return nil, errors.New("превышен лимит регистраций")
	}
	if input.DeviceFingerprint != "" && sameFPCount >= 2 {
		s.log.Warn("registration blocked: too many accounts from fingerprint",
			zap.String("fp", input.DeviceFingerprint), zap.Int("count", sameFPCount))
		return nil, errors.New("превышен лимит регистраций")
	}

	riskDelta := s.anti.ScopeRegistrationRisk(ctx, input.IP, input.DeviceFingerprint, sameIPCount, sameFPCount)

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
		IsAdmin:           s.adminLogin != "" && strings.ToLower(username) == s.adminLogin,
		DeviceFingerprint: &input.DeviceFingerprint,
		LastKnownIP: func() *string {
			if ipOK && input.IP != "" {
				return &input.IP
			}
			return nil
		}(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
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
	Username  string
	Password  string
	IP        string
	UserAgent string
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
		return nil, errors.New("неверный логин или пароль")
	}
	if !password.Verify(*user.PasswordHash, input.Password) {
		return nil, errors.New("неверный логин или пароль")
	}
	if user.IsBanned {
		return nil, errors.New("ваш аккаунт заблокирован")
	}

	s.anti.ResetLoginAttempts(ctx, input.IP+":"+username)

	// Auto-promote to admin if this login is configured as admin and isn't yet (C-5).
	if s.adminLogin != "" && strings.ToLower(username) == s.adminLogin && !user.IsAdmin {
		if err := s.repo.SetAdmin(ctx, user.ID, true); err != nil {
			s.log.Warn("failed to auto-promote admin", zap.Error(err))
		} else {
			user.IsAdmin = true
		}
	}

	// Detect new IP before we overwrite last_known_ip.
	prevIP := ""
	if user.LastKnownIP != nil {
		prevIP = *user.LastKnownIP
	}
	isNewIP := prevIP != "" && input.IP != "" && prevIP != input.IP

	// Update last known IP (best-effort).
	if input.IP != "" {
		_ = s.repo.UpdateLastKnownIP(ctx, user.ID, input.IP)
	}

	// Activity log (best-effort).
	ip := input.IP
	ua := strings.TrimSpace(input.UserAgent)
	var ipPtr *string
	var uaPtr *string
	if ip != "" {
		ipPtr = &ip
	}
	if ua != "" {
		uaPtr = &ua
	}
	_ = s.repo.CreateAccountActivity(ctx, &domain.AccountActivity{
		ID:        uuid.New(),
		UserID:    user.ID,
		EventType: "login",
		IP:        ipPtr,
		UserAgent: uaPtr,
		Details:   nil,
		CreatedAt: time.Now(),
	})

	// Detect new device (user-agent never seen before).
	isNewDevice := false
	if ua != "" {
		if seen, err := s.repo.HasSeenUserAgent(ctx, user.ID, ua); err == nil && !seen {
			isNewDevice = true
		}
	}

	// Notify in Telegram on new IP or new device (best-effort, via worker queue).
	if s.rdb != nil && user.TelegramID != nil && *user.TelegramID != 0 {
		if isNewIP {
			msg := fmt.Sprintf("⚠️ <b>Вход с нового IP</b>\n\nIP: %s\nЕсли это были не вы — срочно смените пароль.", input.IP)
			_ = worker.Enqueue(ctx, s.rdb, worker.QueueNotifyTelegram, worker.NotifyTelegramJob{
				TelegramID: *user.TelegramID,
				Message:    msg,
			})
		}
		if isNewDevice {
			msg := fmt.Sprintf("🖥 <b>Вход с нового устройства</b>\n\nIP: %s\nУстройство: %s\n\nЕсли это были не вы — срочно смените пароль.", input.IP, ua)
			_ = worker.Enqueue(ctx, s.rdb, worker.QueueNotifyTelegram, worker.NotifyTelegramJob{
				TelegramID: *user.TelegramID,
				Message:    msg,
			})
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
		return "", nil, errors.New("неверный тариф подписки")
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
		return "", nil, errors.New("пользователь не найден")
	}

	// Apply active discount promo if present
	discountCode, discountPercent, _ := s.repo.GetActiveDiscount(ctx, userID)
	if discountPercent > 0 {
		reduction := priceKopecks * int64(discountPercent) / 100
		priceKopecks -= reduction
	}

	// ── Free activation (100% discount) ───────────────────────────────────
	// When a promo brings the price to zero we skip Platega entirely and
	// directly create a CONFIRMED payment + enqueue subscription activation.
	if priceKopecks <= 0 {
		freePaymentID := uuid.New()
		freePayment := &domain.Payment{
			ID:             freePaymentID,
			UserID:         userID,
			AmountKopecks:  0,
			Currency:       "RUB",
			Status:         domain.PaymentStatusConfirmed,
			Plan:           plan,
			PaymentMethod:  0, // no payment gateway for free promos
			PlategaPayload: userID.String(),
			RedirectURL:    returnURL,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		if err := s.repo.CreatePayment(ctx, freePayment); err != nil {
			return "", nil, fmt.Errorf("store free payment: %w", err)
		}
		if discountCode != "" {
			_ = s.repo.ClearActiveDiscount(ctx, userID)
		}
		job := worker.PaymentProcessJob{
			TransactionID: freePaymentID.String(),
			UserID:        userID.String(),
			AmountKopecks: 0,
			Plan:          string(plan),
			Status:        string(domain.PaymentStatusConfirmed),
		}
		if err := worker.Enqueue(ctx, s.rdb, worker.QueuePaymentProcess, job); err != nil {
			s.log.Error("enqueue free subscription activation", zap.Error(err))
		}
		s.log.Info("free subscription activated via promo",
			zap.String("user_id", userID.String()),
			zap.String("plan", string(plan)),
			zap.String("payment_id", freePaymentID.String()),
		)
		return returnURL, freePayment, nil
	}

	// Convert kopecks to rubles for Platega (minimum 1 ruble = 100 kopecks)
	if priceKopecks < 100 {
		priceKopecks = 100
	}
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
		ExpiresAt:      func() *time.Time { t := time.Now().Add(30 * time.Minute); return &t }(),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := s.repo.CreatePayment(ctx, payment); err != nil {
		return "", nil, fmt.Errorf("store payment: %w", err)
	}

	// Clear the discount once the payment has been created
	if discountPercent > 0 && discountCode != "" {
		_ = s.repo.ClearActiveDiscount(ctx, userID)
	}

	s.log.Info("payment initiated",
		zap.String("user_id", userID.String()),
		zap.String("plan", string(plan)),
		zap.String("tx_id", txID.String()),
	)
	return platResp.Redirect, payment, nil
}

// GetUserSubscriptions returns all subscriptions for a user.
// For the active subscription the expiry date is refreshed from the
// Remnawave panel so the UI always shows the authoritative value.
func (s *SubscriptionService) GetUserSubscriptions(ctx context.Context, userID uuid.UUID) ([]*domain.Subscription, error) {
	subs, err := s.repo.ListSubscriptions(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Try to fetch the authoritative expiry from Remnawave.
	user, uErr := s.repo.GetByID(ctx, userID)
	if uErr == nil && user != nil && user.RemnaUserUUID != nil && *user.RemnaUserUUID != "" {
		remnaUser, rErr := s.remna.GetUser(ctx, *user.RemnaUserUUID)
		if rErr == nil && remnaUser != nil {
			for _, sub := range subs {
				if sub.Status == domain.SubStatusActive || sub.Status == domain.SubStatusTrial {
					if !remnaUser.ExpireAt.IsZero() {
						sub.ExpiresAt = remnaUser.ExpireAt
					}
					break
				}
			}
		}
	}

	return subs, nil
}

// GetUserActiveSubscription returns the current active subscription, if any.
// The expiry date is refreshed from the Remnawave panel.
func (s *SubscriptionService) GetUserActiveSubscription(ctx context.Context, userID uuid.UUID) (*domain.Subscription, error) {
	sub, err := s.repo.GetActiveSubscription(ctx, userID)
	if err != nil || sub == nil {
		return sub, err
	}

	user, uErr := s.repo.GetByID(ctx, userID)
	if uErr == nil && user != nil && user.RemnaUserUUID != nil && *user.RemnaUserUUID != "" {
		remnaUser, rErr := s.remna.GetUser(ctx, *user.RemnaUserUUID)
		if rErr == nil && remnaUser != nil && !remnaUser.ExpireAt.IsZero() {
			sub.ExpiresAt = remnaUser.ExpireAt
		}
	}
	return sub, nil
}

// InitiateRenewal creates a payment for renewing an active or recently expired subscription.
func (s *SubscriptionService) InitiateRenewal(ctx context.Context, userID uuid.UUID, plan domain.SubscriptionPlan, returnURL string) (string, *domain.Payment, error) {
	return s.InitiatePayment(ctx, userID, plan, returnURL)
}

// InitiateDeviceExpansionPayment creates a Platega payment for +1 device expansion.
func (s *SubscriptionService) InitiateDeviceExpansionPayment(ctx context.Context, userID uuid.UUID, returnURL string) (string, *domain.Payment, error) {
	// Must have an active subscription
	activeSub, err := s.repo.GetActiveSubscription(ctx, userID)
	if err != nil {
		return "", nil, err
	}
	if activeSub == nil || activeSub.ExpiresAt.Before(time.Now()) {
		return "", nil, errors.New("у вас нет активной подписки")
	}

	// Check expansion limit
	existing, err := s.repo.GetActiveDeviceExpansion(ctx, userID)
	if err != nil {
		return "", nil, err
	}
	if existing != nil && existing.ExtraDevices >= domain.DeviceExpansionMaxExtra {
		return "", nil, errors.New("достигнут максимум дополнительных устройств")
	}

	// Check if real money purchases are blocked
	settings, err := s.repo.GetPlatformSettings(ctx)
	if err != nil {
		return "", nil, fmt.Errorf("ошибка server config: %w", err)
	}
	if settings != nil && settings.BlockRealMoneyPurchases {
		return "", nil, errors.New("покупки за деньги временно заблокированы администратором")
	}

	// Re-use existing pending device expansion payment if present
	existingPayment, err := s.repo.GetPendingPaymentByPlan(ctx, userID, domain.PlanDeviceExpansion)
	if err == nil && existingPayment != nil {
		return existingPayment.RedirectURL, existingPayment, nil
	}

	// Rate-limit
	if err := s.anti.CheckAPIRateLimit(ctx, userID.String(), "device_payment_init", 5, time.Hour); err != nil {
		return "", nil, err
	}

	user, err := s.repo.GetByID(ctx, userID)
	if err != nil || user == nil {
		return "", nil, errors.New("пользователь не найден")
	}

	priceKopecks := int64(domain.DeviceExpansionPriceKopecks)
	amountRubles := float64(priceKopecks) / 100.0

	platResp, err := s.platega.CreatePayment(ctx, platega.CreatePaymentRequest{
		PaymentMethod: platega.MethodSBPQR,
		PaymentDetails: platega.PaymentDetails{
			Amount:   amountRubles,
			Currency: "RUB",
		},
		Description: "Расширение лимита устройств +1",
		Return:      returnURL,
		Payload:     userID.String(),
	})
	if err != nil {
		return "", nil, fmt.Errorf("initiate device expansion payment: %w", err)
	}

	txID, err := uuid.Parse(platResp.TransactionID)
	if err != nil {
		return "", nil, fmt.Errorf("invalid platega transaction id: %w", err)
	}

	expiresAt := time.Now().Add(15 * time.Minute)
	payment := &domain.Payment{
		ID:             txID,
		UserID:         userID,
		AmountKopecks:  priceKopecks,
		Currency:       "RUB",
		Status:         domain.PaymentStatusPending,
		Plan:           domain.PlanDeviceExpansion,
		PaymentMethod:  platega.MethodSBPQR,
		PlategaPayload: userID.String(),
		RedirectURL:    platResp.Redirect,
		ExpiresAt:      &expiresAt,
		Idempotency:    platResp.TransactionID,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := s.repo.CreatePayment(ctx, payment); err != nil {
		return "", nil, fmt.Errorf("store device expansion payment: %w", err)
	}

	s.log.Info("device expansion payment initiated",
		zap.String("user_id", userID.String()),
		zap.String("payment_id", txID.String()),
		zap.Int64("amount_kopecks", priceKopecks),
	)
	return platResp.Redirect, payment, nil
}

// GetPendingPayments returns non-expired PENDING payments for a user.
func (s *SubscriptionService) GetPendingPayments(ctx context.Context, userID uuid.UUID) ([]*domain.Payment, error) {
	return s.repo.GetPendingPayments(ctx, userID)
}

// TouchPayment resets the expiry of a PENDING payment to now+30 minutes,
// called when the user returns to the app after visiting the payment page.
// Returns the updated payment (with new expires_at) or an error.
func (s *SubscriptionService) TouchPayment(ctx context.Context, userID, paymentID uuid.UUID) (*domain.Payment, error) {
	ok, err := s.repo.TouchPayment(ctx, userID, paymentID)
	if err != nil {
		return nil, err
	}
	if !ok {
		// Payment not found or not PENDING — return current state from DB.
		return s.GetPaymentByID(ctx, userID, paymentID)
	}
	return s.GetPaymentByID(ctx, userID, paymentID)
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
	repo  *postgres.UserRepo
	remna *remnawave.Client
	anti  *anticheat.Engine
	log   *zap.Logger
}

func NewEconomyService(repo *postgres.UserRepo, remna *remnawave.Client, anti *anticheat.Engine, log *zap.Logger) *EconomyService {
	return &EconomyService{repo: repo, remna: remna, anti: anti, log: log}
}

// CreditYAD safely credits YAD to a user, enforcing daily limits.
func (s *EconomyService) CreditYAD(ctx context.Context, userID uuid.UUID, amount int64, txType domain.YADTxType, refID *uuid.UUID, note string) error {
	if amount <= 0 {
		return errors.New("сумма ЯД должна быть положительной")
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
		return errors.New("сумма списания должна быть положительной")
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
		return nil, errors.New("промокод не найден")
	}
	if promo.ExpiresAt != nil && promo.ExpiresAt.Before(time.Now()) {
		return nil, errors.New("срок действия промокода истёк")
	}
	if promo.UsedCount >= promo.MaxUses {
		return nil, errors.New("промокод уже недоступен")
	}

	used, err := s.repo.HasUserUsedPromo(ctx, promo.ID, userID)
	if err != nil {
		return nil, err
	}
	if used {
		return nil, errors.New("вы уже использовали этот промокод")
	}

	// ── Discount promo: store on user, mark as used, no YAD credited ──────────
	if promo.PromoType == domain.PromoTypeDiscount {
		_, existingPercent, err := s.repo.GetActiveDiscount(ctx, userID)
		if err != nil {
			return nil, err
		}
		if existingPercent > 0 {
			return nil, errors.New("у вас уже есть активная скидка, используйте её перед применением нового промокода")
		}
		tx, err := s.repo.BeginTx(ctx)
		if err != nil {
			return nil, err
		}
		defer tx.Rollback(ctx)
		if err := s.repo.UsePromoCode(ctx, tx, promo.ID, userID); err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		if err := s.repo.SetActiveDiscount(ctx, userID, promo.Code, promo.DiscountPercent); err != nil {
			return nil, err
		}
		s.log.Info("discount promo applied",
			zap.String("user_id", userID.String()),
			zap.String("code", code),
			zap.Int("percent", promo.DiscountPercent),
		)
		return promo, nil
	}

	// ── YAD promo: credit balance ──────────────────────────────────────────────
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
		return nil, errors.New("количество должно быть не менее 1")
	}

	item, err := s.repo.GetShopItemByID(ctx, itemID)
	if err != nil {
		return nil, err
	}
	if item == nil || !item.IsActive {
		return nil, errors.New("товар не найден")
	}
	if item.Stock != -1 && item.Stock < quantity {
		return nil, errors.New("товар закончился")
	}

	totalYAD := item.PriceYAD * int64(quantity)

	user, err := s.repo.GetByID(ctx, userID)
	if err != nil || user == nil {
		return nil, errors.New("пользователь не найден")
	}
	if user.YADBalance < totalYAD {
		return nil, errors.New("недостаточно ЯД на балансе")
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

// BuySubscriptionWithYAD purchases a subscription plan by deducting ЯД from the user's balance.
func (s *EconomyService) BuySubscriptionWithYAD(ctx context.Context, userID uuid.UUID, plan domain.SubscriptionPlan) (*domain.Subscription, error) {
	yadPrice := domain.PlanYADPrice(plan)
	if yadPrice == 0 {
		return nil, errors.New("неизвестный тариф")
	}
	durationDays := domain.PlanDurationDays(plan)

	user, err := s.repo.GetByID(ctx, userID)
	if err != nil || user == nil {
		return nil, errors.New("пользователь не найден")
	}
	if user.YADBalance < yadPrice {
		return nil, errors.New("недостаточно ЯД на балансе")
	}

	now := time.Now()

	// Extend existing active subscription or create new
	activeSub, err := s.repo.GetActiveSubscription(ctx, userID)
	if err != nil {
		return nil, err
	}
	var newExpiry time.Time
	if activeSub != nil && activeSub.ExpiresAt.After(now) {
		newExpiry = activeSub.ExpiresAt.Add(time.Duration(durationDays) * 24 * time.Hour)
	} else {
		newExpiry = now.Add(time.Duration(durationDays) * 24 * time.Hour)
	}

	// ── C-4 fix: deduct YAD and record subscription in DB FIRST, then activate
	// Remnawave. If Remnawave fails after commit, the worker can retry activation;
	// if the TX fails, Remnawave is never touched so the user keeps their balance.

	subID := uuid.New()
	remnaUUID := ""
	if user.RemnaUserUUID != nil {
		remnaUUID = *user.RemnaUserUUID
	}

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Deduct ЯД inside the transaction.
	ref := subID
	if err := s.repo.AdjustYADBalance(ctx, tx, userID, -yadPrice, domain.YADTxSpent, &ref, "Подписка за ЯД: "+string(plan)); err != nil {
		return nil, err
	}

	var sub *domain.Subscription
	if activeSub != nil {
		if err := s.repo.ExtendSubscription(ctx, tx, activeSub.ID, newExpiry); err != nil {
			return nil, err
		}
		activeSub.ExpiresAt = newExpiry
		sub = activeSub
	} else {
		sub = &domain.Subscription{
			ID:           subID,
			UserID:       userID,
			Plan:         plan,
			Status:       domain.SubStatusActive,
			StartsAt:     now,
			ExpiresAt:    newExpiry,
			RemnaSubUUID: &remnaUUID,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := s.repo.CreateSubscription(ctx, tx, sub); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// Activate VPN access AFTER the DB commit so a balance deduction failure
	// never results in free VPN. If Remnawave fails here, the sub record exists
	// and a background retry can re-activate without charging the user again.
	if user.RemnaUserUUID == nil || *user.RemnaUserUUID == "" {
		remnaName := user.RemnaUsername()
		remnaUser, err := s.remna.CreateUser(ctx, remnaName, newExpiry)
		if err != nil {
			// Fallback: if the user already exists in Remnawave (e.g. remna_user_uuid
			// was lost from DB), look them up by username and recover.
			existing, lookupErr := s.remna.GetUserByUsername(ctx, remnaName)
			if lookupErr != nil || existing == nil {
				// Legacy fallback: try UUID-based username from older registrations.
				existing, lookupErr = s.remna.GetUserByUsername(ctx, userID.String())
			}
			if lookupErr != nil || existing == nil {
				s.log.Error("remnawave create user failed after YAD deduction — manual retry needed",
					zap.String("user_id", userID.String()), zap.Error(err))
			} else {
				_ = s.repo.UpdateRemnaUUID(ctx, userID, existing.UUID)
				_ = s.remna.UpdateExpiry(ctx, existing.UUID, newExpiry)
				_ = s.remna.EnableUser(ctx, existing.UUID)
			}
		} else {
			_ = s.repo.UpdateRemnaUUID(ctx, userID, remnaUser.UUID)
		}
	} else {
		if err := s.remna.UpdateExpiry(ctx, remnaUUID, newExpiry); err != nil {
			s.log.Error("remnawave update expiry failed after YAD deduction — manual retry needed",
				zap.String("user_id", userID.String()), zap.Error(err))
		} else {
			_ = s.remna.EnableUser(ctx, remnaUUID)
		}
	}

	s.log.Info("subscription purchased with ЯД",
		zap.String("user_id", userID.String()),
		zap.String("plan", string(plan)),
		zap.Int64("yad_spent", yadPrice),
	)
	return sub, nil
}

// BuyDeviceExpansion purchases a device limit expansion for the user using ЯД.
func (s *EconomyService) BuyDeviceExpansion(ctx context.Context, userID uuid.UUID) (*domain.DeviceExpansion, error) {
	yadPrice := int64(domain.DeviceExpansionPriceYAD)

	user, err := s.repo.GetByID(ctx, userID)
	if err != nil || user == nil {
		return nil, errors.New("пользователь не найден")
	}

	// Must have an active subscription
	activeSub, err := s.repo.GetActiveSubscription(ctx, userID)
	if err != nil {
		return nil, err
	}
	if activeSub == nil || activeSub.ExpiresAt.Before(time.Now()) {
		return nil, errors.New("у вас нет активной подписки")
	}

	if user.YADBalance < yadPrice {
		return nil, errors.New("недостаточно ЯД на балансе")
	}

	// Expiry = end of active subscription
	newExpiry := activeSub.ExpiresAt

	// Check existing active expansion
	existing, err := s.repo.GetActiveDeviceExpansion(ctx, userID)
	if err != nil {
		return nil, err
	}

	newExtra := 1
	if existing != nil {
		if existing.ExtraDevices >= domain.DeviceExpansionMaxExtra {
			return nil, errors.New("достигнут максимум дополнительных устройств")
		}
		newExtra = existing.ExtraDevices + 1
	}

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	ref := uuid.New()
	if err := s.repo.AdjustYADBalance(ctx, tx, userID, -yadPrice, domain.YADTxSpent, &ref, fmt.Sprintf("Расширение устройств: +1 (всего +%d)", newExtra)); err != nil {
		return nil, err
	}

	var expansion *domain.DeviceExpansion
	if existing != nil {
		if err := s.repo.ExtendDeviceExpansion(ctx, tx, existing.ID, newExpiry); err != nil {
			return nil, err
		}
		// Also update extra_devices count
		if err := s.repo.UpdateDeviceExpansionExtra(ctx, tx, existing.ID, newExtra); err != nil {
			return nil, err
		}
		existing.ExtraDevices = newExtra
		existing.ExpiresAt = newExpiry
		expansion = existing
	} else {
		expansion = &domain.DeviceExpansion{
			ID:           ref,
			UserID:       userID,
			ExtraDevices: newExtra,
			ExpiresAt:    newExpiry,
			CreatedAt:    time.Now(),
		}
		if err := s.repo.CreateDeviceExpansion(ctx, tx, expansion); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// Update Remnawave panel device limit
	if user.RemnaUserUUID != nil && *user.RemnaUserUUID != "" {
		newLimit := domain.DeviceMaxPerUser + newExtra
		if err := s.remna.UpdateHwidDeviceLimit(ctx, *user.RemnaUserUUID, newLimit); err != nil {
			s.log.Error("failed to update remnawave hwid device limit after purchase",
				zap.String("user_id", userID.String()),
				zap.Int("new_limit", newLimit),
				zap.Error(err))
		}
	}

	s.log.Info("device expansion purchased (YAD)",
		zap.String("user_id", userID.String()),
		zap.Int("extra_devices", newExtra),
		zap.Int64("yad_spent", yadPrice),
		zap.Time("expires_at", expansion.ExpiresAt),
	)
	return expansion, nil
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
		return nil, errors.New("пользователь не найден")
	}
	if user.TrialUsed {
		return nil, errors.New("пробный период уже был использован")
	}

	now := time.Now()
	expiry := now.Add(3 * 24 * time.Hour)

	// Use the existing Remnawave UUID if the user already has one.
	// For new users we leave it blank until after the commit (C-4: never grant
	// VPN access before the DB record that prevents a second trial is committed).
	remnaUUID := ""
	if user.RemnaUserUUID != nil {
		remnaUUID = *user.RemnaUserUUID
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
	// C-3: mark trial_used inside the same transaction so a crash between
	// CreateSubscription and the old post-commit SetTrialUsed cannot allow
	// the user to claim a second trial.
	if err := s.repo.SetTrialUsedTx(ctx, tx, userID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// Activate VPN access AFTER the commit (C-4). If Remnawave fails, the
	// trial record already exists so the user cannot claim a second trial,
	// and an admin / worker can re-activate them without re-charging.
	if user.RemnaUserUUID == nil || *user.RemnaUserUUID == "" {
		remnaName := user.RemnaUsername()
		remnaUser, err := s.remna.CreateUser(ctx, remnaName, expiry)
		if err != nil {
			// Fallback: if the user already exists in Remnawave, recover UUID.
			existing, lookupErr := s.remna.GetUserByUsername(ctx, remnaName)
			if lookupErr != nil || existing == nil {
				// Legacy fallback: try UUID-based username from older registrations.
				existing, lookupErr = s.remna.GetUserByUsername(ctx, userID.String())
			}
			if lookupErr != nil || existing == nil {
				s.log.Error("remnawave create user failed after trial commit — manual activation needed",
					zap.String("user_id", userID.String()), zap.Error(err))
			} else {
				_ = s.repo.UpdateRemnaUUID(ctx, userID, existing.UUID)
				_ = s.repo.UpdateSubscriptionRemna(ctx, sub.ID, existing.UUID)
				_ = s.remna.UpdateExpiry(ctx, existing.UUID, expiry)
				_ = s.remna.EnableUser(ctx, existing.UUID)
			}
		} else {
			_ = s.repo.UpdateRemnaUUID(ctx, userID, remnaUser.UUID)
			_ = s.repo.UpdateSubscriptionRemna(ctx, sub.ID, remnaUser.UUID)
		}
	} else {
		if err := s.remna.UpdateExpiry(ctx, remnaUUID, expiry); err != nil {
			s.log.Error("remnawave update expiry failed after trial commit",
				zap.String("user_id", userID.String()), zap.Error(err))
		}
		// Re-enable user in case they were previously disabled (expired subscription).
		if err := s.remna.EnableUser(ctx, remnaUUID); err != nil {
			s.log.Warn("remnawave enable user failed after trial commit",
				zap.String("user_id", userID.String()), zap.Error(err))
		}
	}

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

// ─── Device Service ───────────────────────────────────────────────────────────

type DeviceService struct {
	devices *postgres.DeviceRepo
	users   *postgres.UserRepo
	remna   *remnawave.Client
	log     *zap.Logger
}

func NewDeviceService(devices *postgres.DeviceRepo, users *postgres.UserRepo, remna *remnawave.Client, log *zap.Logger) *DeviceService {
	return &DeviceService{devices: devices, users: users, remna: remna, log: log}
}

// ListDevices fetches the user's connected devices from the Remnawave HWID system.
// Falls back to local DB if the user has no remna_user_uuid.
func (s *DeviceService) ListDevices(ctx context.Context, userID uuid.UUID) ([]*domain.Device, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil || user.RemnaUserUUID == nil || *user.RemnaUserUUID == "" {
		// No Remnawave user — return empty list.
		return nil, nil
	}

	resp, err := s.remna.GetUserHwidDevices(ctx, *user.RemnaUserUUID)
	if err != nil {
		s.log.Warn("failed to fetch HWID devices from Remnawave, falling back to local DB",
			zap.String("user_id", userID.String()),
			zap.Error(err))
		return s.devices.ListByUser(ctx, userID)
	}

	devices := make([]*domain.Device, 0, len(resp.Devices))
	for _, d := range resp.Devices {
		name := "Unknown"
		if d.DeviceModel != nil && *d.DeviceModel != "" {
			name = *d.DeviceModel
		} else if d.Platform != nil && *d.Platform != "" {
			name = *d.Platform
		}
		if d.OsVersion != nil && *d.OsVersion != "" {
			name += " (" + *d.OsVersion + ")"
		}

		createdAt, _ := time.Parse(time.RFC3339, d.CreatedAt)
		updatedAt, _ := time.Parse(time.RFC3339, d.UpdatedAt)
		if updatedAt.IsZero() {
			updatedAt = createdAt
		}

		devices = append(devices, &domain.Device{
			ID:         uuid.NewSHA1(uuid.NameSpaceURL, []byte(d.Hwid)),
			UserID:     userID,
			DeviceName: name,
			LastActive: updatedAt,
			CreatedAt:  createdAt,
			IsActive:   true,
			HwidID:     d.Hwid,
		})
	}
	return devices, nil
}

// DisconnectDevice removes a device via the Remnawave HWID API.
func (s *DeviceService) DisconnectDevice(ctx context.Context, userID uuid.UUID, hwidID string) error {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil || user.RemnaUserUUID == nil || *user.RemnaUserUUID == "" {
		return errors.New("устройство не найдено")
	}

	if err := s.remna.DeleteUserHwidDevice(ctx, hwidID, *user.RemnaUserUUID); err != nil {
		return fmt.Errorf("не удалось отключить устройство: %w", err)
	}
	s.log.Info("device disconnected via Remnawave",
		zap.String("user_id", userID.String()),
		zap.String("hwid", hwidID),
	)
	return nil
}
