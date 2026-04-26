// Package service contains all business logic for the VPN platform.
package service

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
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

// clampDiscountPercent normalises a discount value into [0, 100] so a buggy
// admin tool, an unvalidated DB write, or an out-of-range promo can never
// drive priceKopecks negative or inflate it. Defence in depth — the column
// CHECK in migration 024 enforces the same bound at the storage layer.
func clampDiscountPercent(p int) int {
	if p < 0 {
		return 0
	}
	if p > 100 {
		return 100
	}
	return p
}

// isSerializationFailure reports whether err is a PostgreSQL 40001 error,
// which Serializable transactions raise when a write-skew is detected.
// Callers retry the whole transaction on this error.
func isSerializationFailure(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "40001"
	}
	return false
}

// subtleConstantEq is a constant-time string comparator used for
// authenticating tokens (admin bootstrap token, etc.) where a normal `==`
// would leak the matching prefix length to a timing attacker.
func subtleConstantEq(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// sanitizeReturnURL filters the user-supplied `return_url` against the
// configured allow-list of hosts before forwarding it to Platega. Without
// this an attacker could initiate a real payment with `return_url=evil.com`,
// pass the legitimate Platega checkout link to a victim, and redirect the
// victim to a phishing page after a successful payment to our merchant
// account. Returns the sanitized URL or fallback.
func sanitizeReturnURL(raw string, allowedHosts []string, fallback string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return fallback
	}
	host := strings.ToLower(u.Host)
	for _, h := range allowedHosts {
		if strings.EqualFold(strings.TrimSpace(h), host) {
			return u.String()
		}
	}
	return fallback
}

// ─── Auth Service ─────────────────────────────────────────────────────────────

type AuthService struct {
	repo                *postgres.UserRepo
	anti                *anticheat.Engine
	rdb                 *redis.Client
	log                 *zap.Logger
	adminBootstrapToken string // env-only; promotes first registrant to admin if no admin exists
}

func NewAuthService(repo *postgres.UserRepo, anti *anticheat.Engine, rdb *redis.Client, log *zap.Logger, adminBootstrapToken string) *AuthService {
	return &AuthService{
		repo: repo, anti: anti, rdb: rdb, log: log,
		adminBootstrapToken: strings.TrimSpace(adminBootstrapToken),
	}
}

type RegisterInput struct {
	Username          string
	Password          string
	ReferralCode      string // optional
	DeviceFingerprint string
	IP                string
	// BootstrapToken: when non-empty AND it matches the env-configured
	// ADMIN_BOOTSTRAP_TOKEN AND no admin yet exists, the new user is
	// promoted to is_admin=true. After the first admin is created the
	// token has no effect; it MUST be cleared from the env afterwards.
	BootstrapToken string
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

	// Bootstrap-admin: only promotes when (a) the request supplied the
	// expected token, (b) the env-side token is configured, (c) no admin
	// exists yet. Constant-time string compare so the token can't be
	// guessed by timing the response.
	isFirstAdmin := false
	if s.adminBootstrapToken != "" && input.BootstrapToken != "" &&
		subtleConstantEq(input.BootstrapToken, s.adminBootstrapToken) {
		count, _ := s.repo.CountAdmins(ctx)
		if count == 0 {
			isFirstAdmin = true
		}
	}

	user := &domain.User{
		ID:                newUserID,
		Username:          &username,
		PasswordHash:      &hash,
		YADBalance:        0,
		ReferralCode:      refCode,
		RiskScore:         riskDelta,
		IsAdmin:           isFirstAdmin,
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
	// Constant-time path: always run bcrypt, even on user-not-found, so the
	// response time can't be used to enumerate valid usernames. The dummy
	// hash inside VerifyConstantTime burns the same CPU as a real verify.
	hash := ""
	if user != nil && user.PasswordHash != nil {
		hash = *user.PasswordHash
	}
	ok := password.VerifyConstantTime(hash, input.Password)
	if user == nil || !ok {
		s.anti.RecordFailedLogin(ctx, input.IP+":"+username)
		return nil, errors.New("неверный логин или пароль")
	}
	if user.IsBanned {
		return nil, errors.New("ваш аккаунт заблокирован")
	}

	s.anti.ResetLoginAttempts(ctx, input.IP+":"+username)

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
	repo               *postgres.UserRepo
	platega            *platega.Client
	remna              *remnawave.Client
	anti               *anticheat.Engine
	rdb                *redis.Client
	log                *zap.Logger
	allowedReturnHosts []string // lowercased, trimmed
	defaultReturnURL   string   // used when client value is missing/invalid
}

func NewSubscriptionService(
	repo *postgres.UserRepo,
	platega *platega.Client,
	remna *remnawave.Client,
	anti *anticheat.Engine,
	rdb *redis.Client,
	log *zap.Logger,
	allowedReturnHostsCSV string,
	defaultReturnURL string,
) *SubscriptionService {
	hosts := []string{}
	for _, h := range strings.Split(allowedReturnHostsCSV, ",") {
		h = strings.TrimSpace(strings.ToLower(h))
		if h != "" {
			hosts = append(hosts, h)
		}
	}
	if defaultReturnURL == "" {
		defaultReturnURL = "/"
	}
	return &SubscriptionService{
		repo: repo, platega: platega, remna: remna, anti: anti, rdb: rdb, log: log,
		allowedReturnHosts: hosts,
		defaultReturnURL:   defaultReturnURL,
	}
}

// InitiatePayment creates a Platega payment session and a pending Payment record.
// Returns the Platega redirect URL.
func (s *SubscriptionService) InitiatePayment(ctx context.Context, userID uuid.UUID, plan domain.SubscriptionPlan, returnURL string) (string, *domain.Payment, error) {
	basePrice := domain.PlanPriceKopecks(plan)
	if basePrice == 0 {
		return "", nil, errors.New("неверный тариф подписки")
	}

	// Sanitize the user-supplied redirect target against an allow-list.
	// Without this, an attacker can craft a Platega checkout that, after a
	// successful payment to our merchant, redirects the victim to a phishing
	// page styled like our site.
	returnURL = sanitizeReturnURL(returnURL, s.allowedReturnHosts, s.defaultReturnURL)

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

	// Apply active discount promo if present, clamping to a sane range so
	// stray DB writes can never make priceKopecks negative or inflated.
	priceKopecks := basePrice
	discountCode, discountPercent, _ := s.repo.GetActiveDiscount(ctx, userID)
	discountPercent = clampDiscountPercent(discountPercent)
	if discountPercent > 0 {
		reduction := basePrice * int64(discountPercent) / 100
		priceKopecks -= reduction
	}

	// ── Free activation (100% discount) ───────────────────────────────────
	if priceKopecks <= 0 {
		freePaymentID := uuid.New()
		freePayment := &domain.Payment{
			ID:             freePaymentID,
			UserID:         userID,
			AmountKopecks:  0,
			Currency:       "RUB",
			Status:         domain.PaymentStatusConfirmed,
			Plan:           plan,
			PaymentMethod:  0,
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
		Payload:     userID.String(),
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
		// Concurrent /buy for the same (user, plan) lost the race against the
		// partial-unique index payments_pending_user_plan_uq added in
		// migration 024. Fetch the winning row and return that — the user
		// sees a single PENDING checkout instead of two orphan transactions.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			if winner, lookupErr := s.repo.GetPendingPaymentByPlan(ctx, userID, plan); lookupErr == nil && winner != nil {
				s.log.Info("payment-init race deduplicated",
					zap.String("user_id", userID.String()),
					zap.String("plan", string(plan)),
					zap.String("winner_tx", winner.ID.String()))
				return winner.RedirectURL, winner, nil
			}
		}
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
	// Reject unrecognised statuses so we never persist garbage to the DB.
	if !domain.IsValidPaymentStatus(newStatus) {
		s.log.Warn("unexpected payment status from Platega",
			zap.String("payment_id", paymentID.String()),
			zap.String("status", string(platResp.Status)))
		return payment, nil
	}
	if newStatus == payment.Status {
		return payment, nil // nothing changed
	}

	// Persist the new status.
	_ = s.repo.UpdatePaymentStatus(ctx, nil, paymentID, newStatus)
	payment.Status = newStatus

	// If confirmed, enqueue the same processing job the webhook would have sent.
	// Always use the canonical DB amount — never an externally derived value.
	if newStatus == domain.PaymentStatusConfirmed {
		job := worker.PaymentProcessJob{
			TransactionID: paymentID.String(),
			UserID:        userID.String(),
			AmountKopecks: payment.AmountKopecks,
			Plan:          string(payment.Plan),
			Status:        string(newStatus),
		}
		if enqErr := worker.Enqueue(ctx, s.rdb, worker.QueuePaymentProcess, job); enqErr != nil {
			s.log.Error("enqueue payment process after manual check", zap.Error(enqErr))
		}
	}

	return payment, nil
}

// InitiateDeviceExpansionPayment creates a Platega payment for buying extra device slots.
// qty must be 1 or 2. If the user already has an active expansion, the price is the difference.
func (s *SubscriptionService) InitiateDeviceExpansionPayment(ctx context.Context, userID uuid.UUID, qty int, returnURL string) (string, *domain.Payment, error) {
	if qty < 1 || qty > domain.DeviceExpansionMaxExtra {
		return "", nil, fmt.Errorf("количество устройств должно быть 1 или 2")
	}

	returnURL = sanitizeReturnURL(returnURL, s.allowedReturnHosts, s.defaultReturnURL)

	activeSub, err := s.repo.GetActiveSubscription(ctx, userID)
	if err != nil {
		return "", nil, fmt.Errorf("не удалось проверить подписку: %w", err)
	}
	if activeSub == nil {
		return "", nil, errors.New("нет активной подписки — сначала купите подписку")
	}

	existing, err := s.repo.GetActiveDeviceExpansion(ctx, userID)
	if err != nil {
		return "", nil, fmt.Errorf("не удалось проверить расширение: %w", err)
	}
	if existing != nil && existing.ExtraDevices >= qty {
		return "", nil, errors.New("расширение устройств уже активно")
	}

	// Tiered cost based on days remaining.
	daysRemaining := int(time.Until(activeSub.ExpiresAt).Hours() / 24)
	if daysRemaining < 0 {
		daysRemaining = 0
	}
	amountKopecks := domain.DeviceExpansionKopecks(qty, daysRemaining)
	if existing != nil {
		amountKopecks = domain.DeviceExpansionKopecks(qty, daysRemaining) - domain.DeviceExpansionKopecks(existing.ExtraDevices, daysRemaining)
	}

	// Rate limit (separate key from subscription payments).
	rlKey := fmt.Sprintf("rl:initpay:devexp:%s", userID.String())
	cnt, _ := s.rdb.Incr(ctx, rlKey).Result()
	if cnt == 1 {
		s.rdb.Expire(ctx, rlKey, time.Hour)
	}
	if cnt > 5 {
		return "", nil, errors.New("слишком много попыток оплаты — подождите час")
	}

	// Check for existing PENDING payment for same plan.
	plan := domain.PlanDeviceExpansion
	if qty == 2 {
		plan = domain.PlanDeviceExpansion2
	}
	if pending, _ := s.repo.GetPendingPaymentByPlan(ctx, userID, plan); pending != nil {
		return pending.RedirectURL, pending, nil
	}

	amountRubles := float64(amountKopecks) / 100
	desc := fmt.Sprintf("Расширение устройств +%d", qty)

	platResp, err := s.platega.CreatePayment(ctx, platega.CreatePaymentRequest{
		PaymentMethod: platega.MethodSBPQR,
		PaymentDetails: platega.PaymentDetails{
			Amount:   amountRubles,
			Currency: "RUB",
		},
		Description: desc,
		Return:      returnURL,
		FailedURL:   returnURL,
		Payload:     userID.String(),
	})
	if err != nil {
		return "", nil, fmt.Errorf("ошибка платёжного шлюза: %w", err)
	}

	txID, err := uuid.Parse(platResp.TransactionID)
	if err != nil {
		return "", nil, fmt.Errorf("некорректный transaction_id от Platega: %w", err)
	}

	now := time.Now()
	exp := now.Add(30 * time.Minute)
	payment := &domain.Payment{
		ID:            txID,
		UserID:        userID,
		AmountKopecks: amountKopecks,
		Currency:      "RUB",
		Status:        domain.PaymentStatusPending,
		Plan:          plan,
		AddonQty:      qty,
		PaymentMethod: int(platega.MethodSBPQR),
		PlategaPayload: userID.String(),
		RedirectURL:   platResp.Redirect,
		ExpiresAt:     &exp,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := s.repo.CreatePayment(ctx, payment); err != nil {
		// Same partial-unique race as InitiatePayment.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			if winner, lookupErr := s.repo.GetPendingPaymentByPlan(ctx, userID, plan); lookupErr == nil && winner != nil {
				return winner.RedirectURL, winner, nil
			}
		}
		return "", nil, fmt.Errorf("сохранение платежа: %w", err)
	}

	s.log.Info("device expansion payment initiated",
		zap.String("user_id", userID.String()),
		zap.Int("qty", qty),
		zap.Int64("kopecks", amountKopecks))

	return platResp.Redirect, payment, nil
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

	subID := uuid.New()
	remnaUUID := ""
	if user.RemnaUserUUID != nil {
		remnaUUID = *user.RemnaUserUUID
	}

	// All money movement happens in one Serializable transaction with the
	// user row locked FOR UPDATE so two concurrent BuySubscriptionWithYAD
	// calls cannot both read existingSub.ExpiresAt = T and both write
	// expires_at = T + 30d. Retry on 40001 (write-skew detected by Postgres)
	// up to 3 times — that's enough to absorb realistic contention without
	// looping forever on a real bug.
	var sub *domain.Subscription
	var newExpiry time.Time
	for attempt := 0; attempt < 3; attempt++ {
		var err error
		sub, newExpiry, err = s.tryBuyWithYAD(ctx, userID, plan, durationDays, subID, remnaUUID, yadPrice, now)
		if err == nil {
			break
		}
		if isSerializationFailure(err) && attempt < 2 {
			s.log.Info("BuySubscriptionWithYAD serialisation retry",
				zap.String("user_id", userID.String()), zap.Int("attempt", attempt))
			continue
		}
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

// tryBuyWithYAD is the per-attempt body of BuySubscriptionWithYAD. It runs
// inside a fresh Serializable transaction so the caller can simply retry on
// 40001. No Remnawave I/O — that's done after commit.
func (s *EconomyService) tryBuyWithYAD(ctx context.Context, userID uuid.UUID,
	plan domain.SubscriptionPlan, durationDays int, subID uuid.UUID,
	remnaUUID string, yadPrice int64, now time.Time,
) (*domain.Subscription, time.Time, error) {
	tx, err := s.repo.BeginSerializableTx(ctx)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer tx.Rollback(ctx)

	// Lock the user row so the YAD debit and the subscription extension
	// share a single serial point with any concurrent purchase.
	if err := s.repo.LockUserForUpdate(ctx, tx, userID); err != nil {
		return nil, time.Time{}, err
	}

	// Lock the latest subscription row (if any) so the relative SQL extend
	// reads & writes the same expires_at.
	existingSub, err := s.repo.LockLatestSubscriptionForUpdate(ctx, tx, userID)
	if err != nil {
		return nil, time.Time{}, err
	}

	// Deduct YAD atomically (UPDATE … WHERE balance + delta >= 0).
	ref := subID
	if err := s.repo.AdjustYADBalance(ctx, tx, userID, -yadPrice, domain.YADTxSpent, &ref,
		"Подписка за ЯД: "+string(plan)); err != nil {
		return nil, time.Time{}, err
	}

	var sub *domain.Subscription
	var newExpiry time.Time
	if existingSub != nil {
		// Relative extend: GREATEST(expires_at, NOW()) + N days. Atomic on
		// the row, so two concurrent extenders cannot both compute the same
		// target from the same prefetched value.
		newExpiry, err = s.repo.ExtendSubscriptionByDuration(ctx, tx, existingSub.ID, durationDays, plan)
		if err != nil {
			return nil, time.Time{}, err
		}
		existingSub.ExpiresAt = newExpiry
		existingSub.Plan = plan
		existingSub.Status = domain.SubStatusActive
		sub = existingSub
	} else {
		newExpiry = now.Add(time.Duration(durationDays) * 24 * time.Hour)
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
			return nil, time.Time{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, time.Time{}, err
	}
	return sub, newExpiry, nil
}

// BuyDeviceExpansion purchases extra device slots using YAD balance (C-4 pattern).
// qty must be 1 or 2. Upgrade from +1 → +2 is allowed; user pays only the difference.
func (s *EconomyService) BuyDeviceExpansion(ctx context.Context, userID uuid.UUID, qty int) (*domain.DeviceExpansion, error) {
	if qty < 1 || qty > domain.DeviceExpansionMaxExtra {
		return nil, fmt.Errorf("количество устройств должно быть 1 или 2")
	}

	// Must have an active subscription — expansion expires with it.
	activeSub, err := s.repo.GetActiveSubscription(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("не удалось проверить подписку: %w", err)
	}
	if activeSub == nil {
		return nil, errors.New("нет активной подписки — сначала купите подписку")
	}

	user, err := s.repo.GetByID(ctx, userID)
	if err != nil || user == nil {
		return nil, errors.New("пользователь не найден")
	}

	// Tiered cost based on days remaining in the active subscription. The
	// final balance check + debit happens inside the locked Serializable
	// transaction below so a parallel purchase cannot push us into overdraft.
	daysRemaining := int(time.Until(activeSub.ExpiresAt).Hours() / 24)
	if daysRemaining < 0 {
		daysRemaining = 0
	}

	var expansion *domain.DeviceExpansion
	for attempt := 0; attempt < 3; attempt++ {
		exp, err := s.tryBuyDeviceExpansion(ctx, userID, qty, daysRemaining, activeSub.ExpiresAt)
		if err == nil {
			expansion = exp
			break
		}
		if isSerializationFailure(err) && attempt < 2 {
			s.log.Info("BuyDeviceExpansion serialisation retry",
				zap.String("user_id", userID.String()), zap.Int("attempt", attempt))
			continue
		}
		return nil, err
	}
	if expansion == nil {
		return nil, errors.New("не удалось завершить покупку — попробуйте ещё раз")
	}

	// Post-commit: update Remnawave device limit. Use the higher of the
	// requested qty and any pre-existing extra_devices so a concurrent winner
	// of the upsert isn't accidentally downgraded.
	if user.RemnaUserUUID != nil && *user.RemnaUserUUID != "" {
		newLimit := domain.DeviceMaxPerUser + expansion.ExtraDevices
		if err := s.remna.UpdateHwidDeviceLimit(ctx, *user.RemnaUserUUID, newLimit); err != nil {
			s.log.Error("BuyDeviceExpansion: update remnawave device limit",
				zap.String("user_id", userID.String()),
				zap.Int("limit", newLimit),
				zap.Error(err))
		}
	}

	s.log.Info("device expansion purchased via YAD",
		zap.String("user_id", userID.String()),
		zap.Int("qty", qty),
		zap.Int("extra_devices", expansion.ExtraDevices))
	return expansion, nil
}

// tryBuyDeviceExpansion is the per-attempt body of BuyDeviceExpansion. Runs
// inside a fresh Serializable transaction with the user row locked so two
// concurrent purchases serialise on the user, the YAD debit, and the
// device_expansions UPSERT.
func (s *EconomyService) tryBuyDeviceExpansion(ctx context.Context, userID uuid.UUID,
	qty, daysRemaining int, expiresAt time.Time,
) (*domain.DeviceExpansion, error) {
	tx, err := s.repo.BeginSerializableTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := s.repo.LockUserForUpdate(ctx, tx, userID); err != nil {
		return nil, err
	}

	// Re-read existing expansion under the lock so we charge the right delta
	// even if a parallel writer just upgraded the row.
	existing, err := s.repo.GetActiveDeviceExpansion(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("не удалось проверить расширение: %w", err)
	}
	if existing != nil && existing.ExtraDevices >= qty {
		return nil, errors.New("расширение устройств уже активно")
	}

	cost := domain.DeviceExpansionYAD(qty, daysRemaining)
	if existing != nil {
		cost -= domain.DeviceExpansionYAD(existing.ExtraDevices, daysRemaining)
	}
	if cost < 0 {
		cost = 0
	}

	expansion := &domain.DeviceExpansion{
		ID:           uuid.New(),
		UserID:       userID,
		ExtraDevices: qty,
		ExpiresAt:    expiresAt,
		CreatedAt:    time.Now(),
	}

	// AdjustYADBalance refuses to drive the balance below zero atomically,
	// so this serves as the authoritative balance check too. Use the
	// expansion ID as ref_id so the YAD ledger row links back to a concrete
	// expansion record (was previously nil — no audit trail).
	ref := expansion.ID
	if cost > 0 {
		if err := s.repo.AdjustYADBalance(ctx, tx, userID, -cost, domain.YADTxSpent, &ref,
			fmt.Sprintf("расширение устройств +%d", qty)); err != nil {
			return nil, fmt.Errorf("списание ЯД: %w", err)
		}
	}
	if err := s.repo.CreateDeviceExpansion(ctx, tx, expansion); err != nil {
		return nil, fmt.Errorf("создание расширения: %w", err)
	}
	if err := s.repo.IncrementDeviceExpansionCount(ctx, tx, userID); err != nil {
		return nil, fmt.Errorf("обновление счётчика: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return expansion, nil
}

// ExtendDeviceExpansionYAD re-purchases an expansion after subscription renewal.
// It behaves identically to BuyDeviceExpansion but is a named alias for clarity.
func (s *EconomyService) ExtendDeviceExpansionYAD(ctx context.Context, userID uuid.UUID, qty int) (*domain.DeviceExpansion, error) {
	return s.BuyDeviceExpansion(ctx, userID, qty)
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
//
// IDOR defence: we don't trust Remnawave to enforce that `hwidID` belongs
// to `remnaUUID` — that's their concern, not ours. Locally we list the
// user's HWID-tracked devices and verify membership before forwarding the
// delete. Without this, anyone who learned an arbitrary hwid (logs, leaked
// support session, etc.) could disconnect another user's device.
func (s *DeviceService) DisconnectDevice(ctx context.Context, userID uuid.UUID, hwidID string) error {
	if hwidID == "" {
		return errors.New("устройство не найдено")
	}
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil || user.RemnaUserUUID == nil || *user.RemnaUserUUID == "" {
		return errors.New("устройство не найдено")
	}

	devs, err := s.remna.GetUserHwidDevices(ctx, *user.RemnaUserUUID)
	if err != nil {
		return fmt.Errorf("не удалось загрузить список устройств: %w", err)
	}
	owns := false
	for _, d := range devs.Devices {
		if d.Hwid == hwidID {
			owns = true
			break
		}
	}
	if !owns {
		s.log.Warn("device disconnect attempt for non-owned hwid",
			zap.String("user_id", userID.String()),
			zap.String("hwid", hwidID))
		// Same opaque error as 'not found' so we don't leak existence of foreign hwids.
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
