// Package bot implements the Telegram bot interface for the VPN platform.
//
// Commands:
//
// /start [referral_code] — register or login, process referral
// /balance               — show YAD balance + Telegram ID
// /buy                   — buy subscription
// /mysubs                — list subscriptions
// /renew                 — renew subscription
// /promo <code>          — apply promo code
// /referral              — show referral link and stats
// /ticket                — open/manage support tickets
// /trial                 — info about free trial (redirect to site)
// /info                  — info with links to privacy policy and user agreement
// /unlink                — unlink Telegram from the web account
package bot

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"

	"github.com/vpnplatform/internal/domain"
	"github.com/vpnplatform/internal/integration/platega"
	"github.com/vpnplatform/internal/integration/remnawave"
	"github.com/vpnplatform/internal/repository/postgres"
	redisrepo "github.com/vpnplatform/internal/repository/redis"
	"github.com/vpnplatform/internal/service"
	jwtpkg "github.com/vpnplatform/pkg/jwt"
	"github.com/vpnplatform/pkg/password"
)

// ─── Bot struct ───────────────────────────────────────────────────────────────

type Bot struct {
	bot      *tele.Bot
	repo     *postgres.UserRepo
	auth     *service.AuthService
	subSvc   *service.SubscriptionService
	ecoSvc   *service.EconomyService
	trialSvc *service.TrialService
	remna    *remnawave.Client
	jwtMgr   *jwtpkg.Manager
	rdb      *redis.Client
	cfg      BotConfig
	log      *zap.Logger
}

type BotConfig struct {
	Token            string
	AdminID          int64
	WebAppURL        string // e.g. https://yourdomain.com
	PaymentReturnURL string
}

func New(
	cfg BotConfig,
	repo *postgres.UserRepo,
	auth *service.AuthService,
	subSvc *service.SubscriptionService,
	ecoSvc *service.EconomyService,
	trialSvc *service.TrialService,
	remna *remnawave.Client,
	jwtMgr *jwtpkg.Manager,
	rdb *redis.Client,
	log *zap.Logger,
) (*Bot, error) {
	pref := tele.Settings{
		Token:  cfg.Token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}
	b, err := tele.NewBot(pref)
	if err != nil {
		return nil, fmt.Errorf("create telegram bot: %w", err)
	}

	bot := &Bot{
		bot:      b,
		repo:     repo,
		auth:     auth,
		subSvc:   subSvc,
		ecoSvc:   ecoSvc,
		trialSvc: trialSvc,
		remna:    remna,
		jwtMgr:   jwtMgr,
		rdb:      rdb,
		cfg:      cfg,
		log:      log,
	}

	bot.registerHandlers()
	return bot, nil
}

// botRandPassword returns a cryptographically random 32-hex-char password (C-2).
func botRandPassword() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (b *Bot) Start() {
	b.log.Info("telegram bot started")
	b.bot.Start()
}

func (b *Bot) Stop() {
	b.bot.Stop()
}

func (b *Bot) registerHandlers() {
	// M-9: global per-user rate limit — max 20 commands per minute.
	b.bot.Use(func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			if c.Sender() == nil {
				return next(c)
			}
			key := fmt.Sprintf("rl:bot:%d", c.Sender().ID)
			count, err := redisrepo.Increment(context.Background(), b.rdb, key, time.Minute)
			if err == nil && count > 20 {
				return c.Send("⏳ Слишком много запросов. Подождите минуту и попробуйте снова.")
			}
			return next(c)
		}
	})
	b.bot.Handle("/start", b.handleStart)
	b.bot.Handle("/balance", b.handleBalance)
	b.bot.Handle("/buy", b.handleBuy)
	b.bot.Handle("/mysubs", b.handleMySubs)
	b.bot.Handle("/renew", b.handleBuy)
	b.bot.Handle("/promo", b.handlePromo)
	b.bot.Handle("/referral", b.handleReferral)
	b.bot.Handle("/trial", b.handleTrial)
	b.bot.Handle("/ticket", b.handleTicketMenu)
	b.bot.Handle("/help", b.handleHelp)
	b.bot.Handle("/link", b.handleLink)
	b.bot.Handle("/unlink", b.handleUnlink)
	b.bot.Handle("/info", b.handleInfo)
	b.bot.Handle("/resetpassword", b.handleResetPassword)
}

// ─── Link handler ─────────────────────────────────────────────────────────────

// handleLink processes `/link CODE` — links the web account to this Telegram user.
func (b *Bot) handleLink(c tele.Context) error {
	args := strings.Fields(c.Message().Text)
	if len(args) < 2 {
		return c.Send(
			"❌ *Привязка аккаунта*\n\n"+
				"Используйте команду в формате:\n"+
				"`/link КОД`\n\n"+
				"_Откройте сайт → Настройки → «Привязать Telegram» и скопируйте команду._",
			&tele.SendOptions{ParseMode: tele.ModeMarkdown},
		)
	}
	code := strings.ToUpper(strings.TrimSpace(args[1]))
	key := fmt.Sprintf("tg:link:%s", code)

	ctx := context.Background()
	// GetDel atomically reads and removes the code in one round-trip,
	// preventing a second concurrent /link invocation from consuming the same code.
	userIDStr, err := b.rdb.GetDel(ctx, key).Result()
	if err == redis.Nil {
		return c.Send("❌ Код не найден или уже истёк.\n\nЗапросите новый код на сайте.")
	}
	if err != nil {
		b.log.Error("handleLink: redis getdel", zap.Error(err))
		return c.Send("⚠️ Временная ошибка. Попробуйте позже.")
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		b.log.Error("handleLink: invalid user id in redis", zap.String("value", userIDStr))
		return c.Send("⚠️ Внутренняя ошибка. Запросите новый код на сайте.")
	}

	tgID := c.Sender().ID
	tgIDVal := int64(tgID)

	// Check if this telegram_id is already linked to some user
	existingUser, err := b.repo.GetByTelegramID(ctx, tgIDVal)
	if err != nil {
		b.log.Error("handleLink: get by telegram id", zap.Error(err))
		return c.Send("⚠️ Временная ошибка. Попробуйте позже.")
	}

	// If telegram_id is linked to the same user, just confirm
	if existingUser != nil && existingUser.ID == userID {
		return c.Send("✅ Этот Telegram уже привязан к вашему аккаунту!")
	}

	// If telegram_id is linked to another user, unlink it first
	if existingUser != nil && existingUser.ID != userID {
		if err := b.repo.SetTelegramID(ctx, existingUser.ID, nil); err != nil {
			b.log.Error("handleLink: unlink telegram from old user", zap.Error(err))
			return c.Send("⚠️ Не удалось выполнить операцию. Попробуйте позже.")
		}
	}

	// Now link telegram_id to the target user
	if err := b.repo.SetTelegramID(ctx, userID, &tgIDVal); err != nil {
		b.log.Error("handleLink: set telegram id", zap.Error(err))
		return c.Send("⚠️ Не удалось привязать аккаунт. Попробуйте снова.")
	}

	// Activity log (best-effort)
	_ = b.repo.CreateAccountActivity(ctx, &domain.AccountActivity{
		ID:        uuid.New(),
		UserID:    userID,
		EventType: "telegram_link",
		CreatedAt: time.Now(),
	})

	return c.Send(
		"✅ *Telegram успешно привязан!*\n\nТеперь вы можете управлять подпиской прямо из бота.",
		&tele.SendOptions{ParseMode: tele.ModeMarkdown},
	)
}

// ─── Unlink handler ───────────────────────────────────────────────────────────

// handleUnlink processes `/unlink` — generates a one-time code for unlinking
// the Telegram account and sends it to the user. The code must be entered on
// the website to confirm the operation. Rate-limited to 3 per 10 minutes.
func (b *Bot) handleUnlink(c tele.Context) error {
	ctx := context.Background()
	tgID := int64(c.Sender().ID)

	// Rate limit: max 3 requests per 10 minutes per Telegram user.
	rlKey := fmt.Sprintf("rl:unlink_bot:%d", tgID)
	count, rlErr := redisrepo.Increment(ctx, b.rdb, rlKey, 10*time.Minute)
	if rlErr == nil && count > 3 {
		return c.Send("⏳ Слишком много запросов. Повторите через 10 минут.")
	}

	user, err := b.repo.GetByTelegramID(ctx, tgID)
	if err != nil {
		b.log.Error("handleUnlink: get user by tg id", zap.Error(err))
		return c.Send("⚠️ Временная ошибка. Попробуйте позже.")
	}
	if user == nil {
		return c.Send("❌ К этому Telegram не привязан аккаунт.")
	}
	if user.IsBanned {
		return c.Send("🚫 Ваш аккаунт заблокирован.")
	}

	code, err := botRandCode()
	if err != nil {
		b.log.Error("handleUnlink: generate code", zap.Error(err))
		return c.Send("⚠️ Внутренняя ошибка. Попробуйте позже.")
	}

	key := fmt.Sprintf("tg:unlink:%s", code)
	if err := b.rdb.Set(ctx, key, user.ID.String(), 10*time.Minute).Err(); err != nil {
		b.log.Error("handleUnlink: redis set", zap.Error(err))
		return c.Send("⚠️ Временная ошибка. Попробуйте позже.")
	}

	return c.Send(
		"🔓 *Отвязка Telegram*\n\n"+
			"Код для отвязки: `"+code+"`\n\n"+
			"Введите этот код на сайте в разделе\n*Настройки → Отвязать Telegram*.\n\n"+
			"_Код действителен 10 минут._",
		&tele.SendOptions{ParseMode: tele.ModeMarkdown},
	)
}

// botRandCode returns a 6-character alphanumeric code using unambiguous characters.
func botRandCode() (string, error) {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	code := make([]byte, 6)
	for i := range code {
		b := make([]byte, 1)
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
		code[i] = chars[int(b[0])%len(chars)]
	}
	return string(code), nil
}

// ─── Reset password handler ───────────────────────────────────────────────────

// handleResetPassword generates a new random password for the account linked to
// this Telegram user and sends it privately. Rate-limited to 3 per hour.
func (b *Bot) handleResetPassword(c tele.Context) error {
	ctx := context.Background()
	tgID := int64(c.Sender().ID)

	// Rate limit: max 3 resets per hour per Telegram user.
	rlKey := fmt.Sprintf("rl:resetpw:%d", tgID)
	count, rlErr := redisrepo.Increment(ctx, b.rdb, rlKey, time.Hour)
	if rlErr == nil && count > 3 {
		return c.Send("⏳ Слишком много попыток. Повторите через час.")
	}

	user, err := b.repo.GetByTelegramID(ctx, tgID)
	if err != nil {
		b.log.Error("handleResetPassword: get user by tg id", zap.Error(err))
		return c.Send("⚠️ Временная ошибка. Попробуйте позже.")
	}
	if user == nil {
		return c.Send("❌ К этому Telegram не привязан аккаунт.\n\nПривяжите аккаунт на сайте в разделе Настройки → Telegram.")
	}
	if user.IsBanned {
		return c.Send("🚫 Ваш аккаунт заблокирован.")
	}

	newPw, err := botRandPassword()
	if err != nil {
		b.log.Error("handleResetPassword: generate password", zap.Error(err))
		return c.Send("⚠️ Внутренняя ошибка. Попробуйте позже.")
	}

	hash, err := password.Hash(newPw)
	if err != nil {
		b.log.Error("handleResetPassword: hash password", zap.Error(err))
		return c.Send("⚠️ Внутренняя ошибка. Попробуйте позже.")
	}

	if err := b.repo.SetPassword(ctx, user.ID, hash); err != nil {
		b.log.Error("handleResetPassword: set password", zap.Error(err))
		return c.Send("⚠️ Не удалось сбросить пароль. Попробуйте позже.")
	}

	// Invalidate all existing sessions.
	if vErr := redisrepo.SetPasswordVersion(ctx, b.rdb, user.ID.String(), time.Now()); vErr != nil {
		b.log.Warn("handleResetPassword: set password version", zap.Error(vErr))
	}

	// Activity log (best-effort)
	_ = b.repo.CreateAccountActivity(ctx, &domain.AccountActivity{
		ID:        uuid.New(),
		UserID:    user.ID,
		EventType: "password_reset",
		CreatedAt: time.Now(),
	})

	loginName := "—"
	if user.Username != nil {
		loginName = *user.Username
	}

	return c.Send(
		"🔑 *Пароль сброшен!*\n\n"+
			"Логин: `"+loginName+"`\n"+
			"Новый пароль: `"+newPw+"`\n\n"+
			"_Рекомендуем сменить пароль в настройках после входа._\n"+
			"_Все активные сессии завершены._",
		&tele.SendOptions{ParseMode: tele.ModeMarkdown},
	)
}

// ─── Main menu helpers ────────────────────────────────────────────────────────

func mainMenuText(user *domain.User, tgID int64, username string) string {
	name := username
	if name == "" {
		name = fmt.Sprintf("User%d", tgID)
	}
	adminLine := ""
	if user.IsAdmin {
		adminLine = "\n⚙️ _Режим администратора_"
	}
	return fmt.Sprintf(
		"🐍 *MelloWPN*\n\n"+
			"Привет, *%s*! 👋\n"+
			"━━━━━━━━━━━━━━━━━\n"+
			"💎 Баланс: *%d ЯД* (~%.0f ₽)\n"+
			"🪪 ID: `%d`%s\n"+
			"━━━━━━━━━━━━━━━━━\n"+
			"Выберите действие 👇",
		name,
		user.YADBalance,
		float64(user.YADBalance)*2.5,
		tgID,
		adminLine,
	)
}

func (b *Bot) mainMenuMarkup(user *domain.User) *tele.ReplyMarkup {
	rm := &tele.ReplyMarkup{}

	btnBuy := rm.Data("💎 Купить VPN", "menu_buy")
	btnMySubs := rm.Data("📋 Мои подписки", "menu_mysubs")
	btnTrial := rm.Data("🆓 Пробный период", "menu_trial")
	btnPromo := rm.Data("🎟 Промокод", "menu_promo")
	btnRef := rm.Data("👥 Рефералы", "menu_referrals")
	btnYad := rm.Data("🛒 Магазин ЯД", "menu_yadshop")
	btnHelp := rm.Data("❓ Помощь", "menu_help")
	btnSupport := rm.Data("🔧 Поддержка", "menu_support")
	btnInfo := rm.Data("ℹ️ О сервисе", "menu_info")

	rows := []tele.Row{
		rm.Row(btnBuy, btnMySubs),
		rm.Row(btnTrial, btnPromo),
		rm.Row(btnRef, btnYad),
		rm.Row(btnHelp, btnSupport),
		rm.Row(btnInfo),
	}
	if user.IsAdmin {
		btnAdmin := rm.Data("⚙️ Админ-панель", "menu_admin")
		rows = append(rows, rm.Row(btnAdmin))
	}
	rm.Inline(rows...)
	return rm
}

func backBtn(rm *tele.ReplyMarkup) tele.Btn {
	return rm.Data("🏠 Главное меню", "menu_back")
}

func (b *Bot) sendMainMenu(c tele.Context) error {
	ctx := context.Background()
	user, err := b.getUser(ctx, c)
	if err != nil {
		return c.Send("❌ " + err.Error())
	}
	tgID := int64(c.Sender().ID)
	username := c.Sender().Username
	return c.Send(
		mainMenuText(user, tgID, username),
		&tele.SendOptions{ParseMode: tele.ModeMarkdown},
		b.mainMenuMarkup(user),
	)
}

// ─── /start ───────────────────────────────────────────────────────────────────

func (b *Bot) handleStart(c tele.Context) error {
	ctx := context.Background()
	tgID := c.Sender().ID
	username := c.Sender().Username

	user, err := b.repo.GetByTelegramID(ctx, int64(tgID))
	if err != nil {
		return c.Send("❌ Внутренняя ошибка. Попробуйте позже.")
	}

	if user != nil {
		if user.IsBanned {
			return c.Send("🚫 Ваш аккаунт заблокирован.")
		}
		return c.Send(
			mainMenuText(user, int64(tgID), username),
			&tele.SendOptions{ParseMode: tele.ModeMarkdown},
			b.mainMenuMarkup(user),
		)
	}

	// New user — parse referral code from /start <referralCode>
	referralCode := ""
	parts := strings.Fields(c.Text())
	if len(parts) > 1 {
		referralCode = parts[1]
	}

	login := fmt.Sprintf("tg%d", tgID)
	randPass, err := botRandPassword()
	if err != nil {
		b.log.Error("bot: generate random password failed", zap.Error(err))
		return c.Send("❌ Внутренняя ошибка. Попробуйте позже.")
	}

	newUser, err := b.auth.Register(ctx, service.RegisterInput{
		Username:     login,
		Password:     randPass,
		ReferralCode: referralCode,
		IP:           "",
	})
	if err != nil {
		if existing, lookupErr := b.repo.GetByUsername(ctx, login); lookupErr == nil && existing != nil && existing.TelegramID == nil {
			tgIDVal := int64(tgID)
			_ = b.repo.SetTelegramID(ctx, existing.ID, &tgIDVal)
			return c.Send(
				mainMenuText(existing, int64(tgID), username),
				&tele.SendOptions{ParseMode: tele.ModeMarkdown},
				b.mainMenuMarkup(existing),
			)
		}
		b.log.Warn("bot registration failed", zap.Error(err), zap.Int64("tg_id", int64(tgID)))
		return c.Send("❌ Не удалось создать аккаунт. Попробуйте позже.")
	}

	tgIDVal := int64(tgID)
	if err := b.repo.SetTelegramID(ctx, newUser.ID, &tgIDVal); err != nil {
		b.log.Warn("set telegram id after registration", zap.Error(err))
	}

	// Activity log (best-effort)
	_ = b.repo.CreateAccountActivity(ctx, &domain.AccountActivity{
		ID:        uuid.New(),
		UserID:    newUser.ID,
		EventType: "registration",
		CreatedAt: time.Now(),
	})

	name := username
	if name == "" {
		name = fmt.Sprintf("User%d", tgID)
	}
	_ = c.Send(
		fmt.Sprintf(
			"🎉 *Добро пожаловать в MelloWPN!*\n\n"+
				"Привет, *%s*! Аккаунт создан ✅\n\n"+
				"🆓 Попробуйте VPN бесплатно — активируйте пробный период\n"+
				"💎 Или выберите тариф от *40 ₽/неделю*\n"+
				"👥 Приглашайте друзей — получайте *15%%* бонус с каждого платежа\n\n"+
				"Выберите действие в меню ниже 👇",
			name,
		),
		&tele.SendOptions{ParseMode: tele.ModeMarkdown},
	)
	return c.Send(
		mainMenuText(newUser, int64(tgID), username),
		&tele.SendOptions{ParseMode: tele.ModeMarkdown},
		b.mainMenuMarkup(newUser),
	)
}

// ─── /balance ─────────────────────────────────────────────────────────────────

func (b *Bot) handleBalance(c tele.Context) error {
	ctx := context.Background()
	user, err := b.getUser(ctx, c)
	if err != nil {
		return c.Send("❌ " + err.Error())
	}

	tgID := c.Sender().ID
	rm := &tele.ReplyMarkup{}
	btnBuyInline := rm.Data("💎 Купить VPN", "menu_buy")
	btnYad := rm.Data("🛒 Магазин яда", "menu_yadshop")
	rm.Inline(
		rm.Row(btnBuyInline, btnYad),
		rm.Row(backBtn(rm)),
	)

	return c.Send(fmt.Sprintf(
		"💰 *Кошелёк*\n"+
			"━━━━━━━━━━━━━━━━━\n\n"+
			"🏆 Баланс: *%d ЯД* (~%.0f ₽)\n"+
			"📊 Курс: 1 ЯД ≈ 2.5 ₽\n\n"+
			"🪪 Telegram ID: `%d`\n\n"+
			"━━━━━━━━━━━━━━━━━\n"+
			"💡 *Как заработать ЯД:*\n"+
			"• 👥 Приглашайте друзей — *15%%* с платежей\n"+
			"• 💎 Покупайте подписки — бонус ЯД\n"+
			"• 🎟 Используйте промокоды",
		user.YADBalance,
		float64(user.YADBalance)*2.5,
		tgID,
	), &tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm)
}

// ─── /buy ─────────────────────────────────────────────────────────────────────

func (b *Bot) handleBuy(c tele.Context) error {
	ctx := context.Background()
	_, err := b.getUser(ctx, c)
	if err != nil {
		return c.Send("❌ " + err.Error())
	}

	rm := &tele.ReplyMarkup{}
	btnWeek := rm.Data("📅 1 неделя — 40 ₽", "buy_1week")
	btnMonth := rm.Data("📆 1 месяц — 100 ₽", "buy_1month")
	btnThree := rm.Data("🗓 3 месяца — 270 ₽ 🔥", "buy_3months")
	rm.Inline(
		rm.Row(btnWeek),
		rm.Row(btnMonth),
		rm.Row(btnThree),
		rm.Row(backBtn(rm)),
	)

	return c.Send(
		"💎 *Купить VPN*\n"+
			"━━━━━━━━━━━━━━━━━\n\n"+
			"📅 *1 неделя* — 40 ₽\n"+
			"    ↳ 5.7 ₽/день · +5 ЯД бонус\n\n"+
			"📆 *1 месяц* — 100 ₽  ⭐️\n"+
			"    ↳ 3.3 ₽/день · +15 ЯД бонус\n\n"+
			"🗓 *3 месяца* — 270 ₽  🔥\n"+
			"    ↳ 3.0 ₽/день · +50 ЯД бонус\n\n"+
			"━━━━━━━━━━━━━━━━━\n"+
			"💳 Оплата через СБП\n"+
			"🔄 Без автопродления",
		&tele.SendOptions{ParseMode: tele.ModeMarkdown},
		rm,
	)
}

func (b *Bot) handleBuyCallback(plan domain.SubscriptionPlan) tele.HandlerFunc {
	return func(c tele.Context) error {
		ctx := context.Background()
		_ = c.Respond()
		user, err := b.getUser(ctx, c)
		if err != nil {
			return c.Send("❌ " + err.Error())
		}

		redirect, payment, err := b.subSvc.InitiatePayment(ctx, user.ID, plan, b.cfg.PaymentReturnURL)
		if err != nil {
			return c.Send("❌ " + err.Error())
		}

		rm := &tele.ReplyMarkup{}
		btnPay := rm.URL("💳 Перейти к оплате", redirect)
		rm.Inline(
			rm.Row(btnPay),
			rm.Row(backBtn(rm)),
		)

		return c.Send(fmt.Sprintf(
			"💳 *Оплата подписки*\n"+
				"━━━━━━━━━━━━━━━━━\n\n"+
				"📋 Тариф: *%s*\n"+
				"💰 Сумма: *%.0f ₽*\n"+
				"🆔 Платёж: `%s`\n\n"+
				"Нажмите кнопку ниже для перехода к оплате.\n"+
				"_Ссылка действительна 15 минут._",
			planName(plan),
			float64(payment.AmountKopecks)/100,
			payment.ID.String(),
		), &tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm)
	}
}

// ─── /mysubs ──────────────────────────────────────────────────────────────────

func (b *Bot) handleMySubs(c tele.Context) error {
	ctx := context.Background()
	user, err := b.getUser(ctx, c)
	if err != nil {
		return c.Send("❌ " + err.Error())
	}

	subs, err := b.subSvc.GetUserSubscriptions(ctx, user.ID)

	rm := &tele.ReplyMarkup{}
	if err != nil || len(subs) == 0 {
		btnBuyInline := rm.Data("💎 Купить VPN", "menu_buy")
		rm.Inline(
			rm.Row(btnBuyInline),
			rm.Row(backBtn(rm)),
		)
		return c.Send(
			"📋 *Мои подписки*\n\n"+
				"📭 У вас пока нет активных подписок.\n\n"+
				"Попробуйте бесплатный пробный период или купите тариф!",
			&tele.SendOptions{ParseMode: tele.ModeMarkdown},
			rm,
		)
	}

	msg := "📋 *Мои подписки*\n━━━━━━━━━━━━━━━━━\n"
	for i, sub := range subs {
		statusEmoji := "🟢"
		statusText := "Активна"
		switch sub.Status {
		case domain.SubStatusExpired:
			statusEmoji = "🔴"
			statusText = "Истекла"
		case domain.SubStatusTrial:
			statusEmoji = "🆓"
			statusText = "Пробная"
		case domain.SubStatusCanceled:
			statusEmoji = "⚫"
			statusText = "Отменена"
		}
		daysLeft := int(time.Until(sub.ExpiresAt).Hours() / 24)
		daysStr := ""
		if sub.Status == domain.SubStatusActive || sub.Status == domain.SubStatusTrial {
			if daysLeft > 0 {
				daysStr = fmt.Sprintf("\n   ⏳ Осталось: *%d дн.*", daysLeft)
			} else {
				daysStr = "\n   ⏰ _Подписка закончилась_"
			}
		}
		msg += fmt.Sprintf(
			"\n%s %d. *%s* — %s\n   📅 До: `%s`%s\n",
			statusEmoji, i+1, planName(sub.Plan), statusText,
			sub.ExpiresAt.Format("02.01.2006"), daysStr,
		)
	}

	btnRenew := rm.Data("🔄 Продлить подписку", "menu_buy")
	btnSubscribeLink := rm.Data("🔗 Подключить VPN", "get_vpn_link")
	rm.Inline(
		rm.Row(btnSubscribeLink),
		rm.Row(btnRenew),
		rm.Row(backBtn(rm)),
	)
	return c.Send(msg, &tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm)
}

// ─── /promo ───────────────────────────────────────────────────────────────────

func (b *Bot) handlePromo(c tele.Context) error {
	ctx := context.Background()
	user, err := b.getUser(ctx, c)
	if err != nil {
		return c.Send("❌ " + err.Error())
	}

	parts := strings.Fields(c.Text())
	if len(parts) < 2 {
		rm := &tele.ReplyMarkup{}
		rm.Inline(rm.Row(backBtn(rm)))
		return c.Send(
			"🎟 *Промокод*\n\n"+
				"Отправьте команду в формате:\n"+
				"`/promo КОД`\n\n"+
				"Пример: `/promo SUMMER2024`",
			&tele.SendOptions{ParseMode: tele.ModeMarkdown},
			rm,
		)
	}
	code := strings.ToUpper(parts[1])

	promo, err := b.ecoSvc.UsePromoCode(ctx, user.ID, code)
	if err != nil {
		rm := &tele.ReplyMarkup{}
		rm.Inline(rm.Row(backBtn(rm)))
		return c.Send("❌ "+err.Error(), rm)
	}

	rm := &tele.ReplyMarkup{}
	rm.Inline(rm.Row(backBtn(rm)))
	return c.Send(fmt.Sprintf(
		"🎟 *Промокод активирован!*\n\n"+
			"🏆 Начислено: *%d ЯД*\n"+
			"💸 Эквивалент: *%.0f ₽*",
		promo.YADAmount,
		float64(promo.YADAmount)*2.5,
	), &tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm)
}

// ─── /referral ────────────────────────────────────────────────────────────────

func (b *Bot) handleReferral(c tele.Context) error {
	ctx := context.Background()
	user, err := b.getUser(ctx, c)
	if err != nil {
		return c.Send("❌ " + err.Error())
	}

	refs, _ := b.repo.GetReferralsByReferrer(ctx, user.ID)
	referralLink := fmt.Sprintf("https://t.me/%s?start=%s", b.bot.Me.Username, user.ReferralCode)

	shareURL := fmt.Sprintf(
		"https://t.me/share/url?url=%s&text=VPN%%20платформа%%20—%%20быстрый%%20и%%20безопасный%%20VPN",
		referralLink,
	)

	rm := &tele.ReplyMarkup{}
	btnShare := rm.URL("📤 Поделиться ссылкой", shareURL)
	rm.Inline(
		rm.Row(btnShare),
		rm.Row(backBtn(rm)),
	)

	return c.Send(fmt.Sprintf(
		"👥 *Реферальная программа*\n"+
			"━━━━━━━━━━━━━━━━━\n\n"+
			"🔗 Ваша ссылка:\n`%s`\n\n"+
			"📊 Приглашено: *%d* чел.\n\n"+
			"💎 *Как это работает:*\n"+
			"Друг покупает подписку → вы получаете *15%%* в ЯД\n\n"+
			"💰 *Выплата бонуса:*\n"+
			"• 30%% — сразу после оплаты\n"+
			"• 70%% — через 30 дней\n\n"+
			"📤 _Поделитесь ссылкой через кнопку ниже_",
		referralLink,
		len(refs),
	), &tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm)
}

// ─── /trial ───────────────────────────────────────────────────────────────────

func (b *Bot) handleTrial(c tele.Context) error {
	webURL := b.cfg.WebAppURL
	if webURL == "" {
		webURL = "https://vpn-platform.ru"
	}

	rm := &tele.ReplyMarkup{}
	btnRegister := rm.URL("🌐 Открыть сайт", webURL+"/register")
	rm.Inline(
		rm.Row(btnRegister),
		rm.Row(backBtn(rm)),
	)

	return c.Send(
		"🆓 *Пробный период*\n"+
			"━━━━━━━━━━━━━━━━━\n\n"+
			"Попробуйте VPN бесплатно!\n\n"+
			"📌 *Что нужно сделать:*\n"+
			"1️⃣ Зарегистрируйтесь на сайте\n"+
			"2️⃣ Перейдите в *«Подписки»*\n"+
			"3️⃣ Нажмите *«Активировать пробный период»*\n\n"+
			"✅ Без оплаты · Без привязки карты",
		&tele.SendOptions{ParseMode: tele.ModeMarkdown},
		rm,
	)
}

// ─── /ticket ──────────────────────────────────────────────────────────────────

func (b *Bot) handleTicketMenu(c tele.Context) error {
	ctx := context.Background()
	user, err := b.getUser(ctx, c)
	if err != nil {
		return c.Send("❌ " + err.Error())
	}

	tickets, _ := b.repo.ListTickets(ctx, &user.ID, "open", 5, 0)
	rm := &tele.ReplyMarkup{}
	rm.Inline(rm.Row(backBtn(rm)))

	if len(tickets) == 0 {
		return c.Send(
			"🔧 *Поддержка*\n"+
				"━━━━━━━━━━━━━━━━━\n\n"+
				"📭 Открытых обращений нет.\n\n"+
				"Создать тикет → перейдите на сайт в раздел *«Поддержка»*.\n"+
				"Или напишите: @Mellow\\_support",
			&tele.SendOptions{ParseMode: tele.ModeMarkdown},
			rm,
		)
	}

	msg := "🔧 *Поддержка*\n━━━━━━━━━━━━━━━━━\n\n"
	for _, t := range tickets {
		statusEmoji := "🟢"
		if t.Status == "closed" {
			statusEmoji = "🔴"
		} else if t.Status == "answered" {
			statusEmoji = "🟡"
		}
		msg += fmt.Sprintf("%s `%s` — %s\n", statusEmoji, t.ID.String()[:8], t.Subject)
	}
	return c.Send(msg, &tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm)
}

// ─── /help ────────────────────────────────────────────────────────────────────

func (b *Bot) handleHelp(c tele.Context) error {
	rm := &tele.ReplyMarkup{}
	btnBuy := rm.Data("💎 Купить VPN", "menu_buy")
	btnSubs := rm.Data("📋 Мои подписки", "menu_mysubs")
	rm.Inline(
		rm.Row(btnBuy, btnSubs),
		rm.Row(backBtn(rm)),
	)
	return c.Send(
		"❓ *Помощь*\n"+
			"━━━━━━━━━━━━━━━━━\n\n"+
			"🔹 *Подписка и VPN:*\n"+
			"  /buy — купить подписку\n"+
			"  /mysubs — список подписок\n"+
			"  /trial — бесплатный пробный период\n\n"+
			"🔹 *Кошелёк и бонусы:*\n"+
			"  /balance — баланс ЯД\n"+
			"  /referral — реферальная программа\n"+
			"  /promo КОД — активировать промокод\n\n"+
			"🔹 *Аккаунт:*\n"+
			"  /resetpassword — сбросить пароль\n"+
			"  /link КОД — привязать Telegram\n"+
			"  /unlink — отвязать Telegram\n\n"+
			"🔹 *Прочее:*\n"+
			"  /ticket — поддержка\n"+
			"  /info — документы и контакты\n\n"+
			"_Вопросы? Пишите в /ticket или @Mellow\\_support_",
		&tele.SendOptions{ParseMode: tele.ModeMarkdown},
		rm,
	)
}

// ─── /info ────────────────────────────────────────────────────────────────────

func (b *Bot) handleInfo(c tele.Context) error {
	rm := &tele.ReplyMarkup{}

	btnPrivacy := rm.URL("🔒 Политика конфиденциальности", b.cfg.WebAppURL+"/PrivacyPolicy")
	btnAgreement := rm.URL("📋 Пользовательское соглашение", b.cfg.WebAppURL+"/UserAgreement")
	btnSupport := rm.URL("💬 Поддержка: @Mellow_support", "https://t.me/Mellow_support")
	btnBack := backBtn(rm)

	rm.Inline(
		rm.Row(btnPrivacy),
		rm.Row(btnAgreement),
		rm.Row(btnSupport),
		rm.Row(btnBack),
	)

	return c.Send(
		"ℹ️ *О сервисе MelloWPN*\n"+
			"━━━━━━━━━━━━━━━━━\n\n"+
			"🛡 Протокол: VLESS/Reality\n"+
			"📱 Все платформы: iOS, Android, Windows, macOS, Linux\n"+
			"🚫 Без рекламы и трекеров\n\n"+
			"💬 Поддержка: @Mellow\\_support\n\n"+
			"Документы доступны по кнопкам ниже:",
		&tele.SendOptions{ParseMode: tele.ModeMarkdown},
		rm,
	)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func (b *Bot) getUser(ctx context.Context, c tele.Context) (*domain.User, error) {
	tgID := int64(c.Sender().ID)
	user, err := b.repo.GetByTelegramID(ctx, tgID)
	if err != nil {
		return nil, fmt.Errorf("ошибка базы данных")
	}
	if user == nil {
		return nil, fmt.Errorf("аккаунт не найден. Используйте /start для регистрации")
	}
	if user.IsBanned {
		return nil, fmt.Errorf("ваш аккаунт заблокирован")
	}
	return user, nil
}

func usernameOrID(username string, id int64) string {
	if username != "" {
		return "@" + username
	}
	return fmt.Sprintf("User%d", id)
}

func planName(p domain.SubscriptionPlan) string {
	switch p {
	case domain.PlanWeek:
		return "1 неделя"
	case domain.PlanMonth:
		return "1 месяц"
	case domain.PlanThreeMonth:
		return "3 месяца"
	default:
		return string(p)
	}
}

// SendNotification sends a message to a user by Telegram ID (called from worker)
func (b *Bot) SendNotification(tgID int64, message string) error {
	_, err := b.bot.Send(&tele.User{ID: tgID}, message)
	return err
}

// ─── RegisterBuyCallbacks ─────────────────────────────────────────────────────

func (b *Bot) RegisterBuyCallbacks() {
	// Buy plan callbacks
	b.bot.Handle(&tele.Btn{Unique: "buy_1week"}, b.handleBuyCallback(domain.PlanWeek))
	b.bot.Handle(&tele.Btn{Unique: "buy_1month"}, b.handleBuyCallback(domain.PlanMonth))
	b.bot.Handle(&tele.Btn{Unique: "buy_3months"}, b.handleBuyCallback(domain.PlanThreeMonth))

	// Main menu callbacks
	b.bot.Handle(&tele.Btn{Unique: "menu_trial"}, func(c tele.Context) error {
		_ = c.Respond()
		return b.handleTrial(c)
	})
	b.bot.Handle(&tele.Btn{Unique: "menu_buy"}, func(c tele.Context) error {
		_ = c.Respond()
		return b.handleBuy(c)
	})
	b.bot.Handle(&tele.Btn{Unique: "menu_mysubs"}, func(c tele.Context) error {
		_ = c.Respond()
		return b.handleMySubs(c)
	})
	b.bot.Handle(&tele.Btn{Unique: "menu_promo"}, func(c tele.Context) error {
		_ = c.Respond()
		rm := &tele.ReplyMarkup{}
		rm.Inline(rm.Row(backBtn(rm)))
		return c.Send(
			"🎟 *Промокод*\n\n"+
				"Отправьте команду в формате:\n"+
				"`/promo КОД`\n\n"+
				"Пример: `/promo SUMMER2024`",
			&tele.SendOptions{ParseMode: tele.ModeMarkdown},
			rm,
		)
	})
	b.bot.Handle(&tele.Btn{Unique: "menu_support"}, func(c tele.Context) error {
		_ = c.Respond()
		return b.handleTicketMenu(c)
	})
	b.bot.Handle(&tele.Btn{Unique: "menu_help"}, func(c tele.Context) error {
		_ = c.Respond()
		return b.handleHelp(c)
	})
	b.bot.Handle(&tele.Btn{Unique: "menu_referrals"}, func(c tele.Context) error {
		_ = c.Respond()
		return b.handleReferral(c)
	})
	b.bot.Handle(&tele.Btn{Unique: "menu_info"}, func(c tele.Context) error {
		_ = c.Respond()
		return b.handleInfo(c)
	})
	b.bot.Handle(&tele.Btn{Unique: "menu_yadshop"}, func(c tele.Context) error {
		_ = c.Respond()
		ctx := context.Background()
		user, err := b.getUser(ctx, c)
		if err != nil {
			return c.Send("❌ " + err.Error())
		}
		webURL := b.cfg.WebAppURL
		if webURL == "" {
			webURL = "https://vpn-platform.ru"
		}
		rm := &tele.ReplyMarkup{}
		btnShop := rm.URL("🛒 Открыть магазин", webURL+"/shop")
		rm.Inline(
			rm.Row(btnShop),
			rm.Row(backBtn(rm)),
		)
		return c.Send(fmt.Sprintf(
			"🛒 *Магазин ЯД*\n"+
				"━━━━━━━━━━━━━━━━━\n\n"+
				"💎 Ваш баланс: *%d ЯД* (~%.0f ₽)\n\n"+
				"Покупайте подписки за ЯД — дешевле, чем за рубли!\n\n"+
				"📅 1 неделя — *%d ЯД*\n"+
				"📆 1 месяц — *%d ЯД*  ⭐️\n"+
				"🗓 3 месяца — *%d ЯД*  🔥\n\n"+
				"_Откройте магазин на сайте:_",
			user.YADBalance,
			float64(user.YADBalance)*2.5,
			domain.PlanYADPrice(domain.PlanWeek),
			domain.PlanYADPrice(domain.PlanMonth),
			domain.PlanYADPrice(domain.PlanThreeMonth),
		), &tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm)
	})
	b.bot.Handle(&tele.Btn{Unique: "menu_admin"}, func(c tele.Context) error {
		_ = c.Respond()
		ctx := context.Background()
		user, err := b.getUser(ctx, c)
		if err != nil || !user.IsAdmin {
			return c.Respond(&tele.CallbackResponse{Text: "Доступ запрещён"})
		}
		webURL := b.cfg.WebAppURL
		if webURL == "" {
			webURL = "https://vpn-platform.ru"
		}
		rm := &tele.ReplyMarkup{}
		btnPanel := rm.URL("🖥 Открыть панель", webURL+"/admin")
		rm.Inline(
			rm.Row(btnPanel),
			rm.Row(backBtn(rm)),
		)
		return c.Send(
			"⚙️ *Панель администратора*\n\nОткройте веб-панель для управления платформой.",
			&tele.SendOptions{ParseMode: tele.ModeMarkdown},
			rm,
		)
	})
	b.bot.Handle(&tele.Btn{Unique: "menu_back"}, func(c tele.Context) error {
		_ = c.Respond()
		return b.sendMainMenu(c)
	})

	// VPN subscription link handler (same logic as GetConnection in HTTP handler)
	b.bot.Handle(&tele.Btn{Unique: "get_vpn_link"}, func(c tele.Context) error {
		_ = c.Respond()
		ctx := context.Background()
		user, err := b.getUser(ctx, c)
		if err != nil {
			return c.Send("❌ " + err.Error())
		}

		remnaUUID := ""
		if user.RemnaUserUUID != nil {
			remnaUUID = *user.RemnaUserUUID
		}

		// Lazy repair: if remna_user_uuid is missing, try to recover it from the
		// subscription's remna_sub_uuid (which is the same Remnawave user UUID).
		if remnaUUID == "" {
			subs, subErr := b.repo.GetActiveSubscription(ctx, user.ID)
			if subErr != nil || subs == nil {
				return c.Send("❌ Активная подписка не найдена.\n\nКупите подписку, чтобы использовать VPN.")
			}

			// Path 1: subscription has a stored remna_sub_uuid — use it directly
			if subs.RemnaSubUUID != nil && *subs.RemnaSubUUID != "" {
				remnaUUID = *subs.RemnaSubUUID
				_ = b.repo.UpdateRemnaUUID(ctx, user.ID, remnaUUID)
			} else {
				// Path 2: look up by username in Remnawave
				remnaName := user.RemnaUsername()
				remnaUser, lookupErr := b.remna.GetUserByUsername(ctx, remnaName)
				if lookupErr != nil || remnaUser == nil || remnaUser.UUID == "" {
					// Legacy fallback: try UUID-based username from older registrations.
					remnaUser, lookupErr = b.remna.GetUserByUsername(ctx, user.ID.String())
				}
				if lookupErr == nil && remnaUser != nil && remnaUser.UUID != "" {
					remnaUUID = remnaUser.UUID
					_ = b.repo.UpdateRemnaUUID(ctx, user.ID, remnaUUID)
					rm := &tele.ReplyMarkup{}
					btnCopy := rm.URL("📋 Открыть ссылку", remnaUser.SubscribeURL)
					rm.Inline(
						rm.Row(btnCopy),
						rm.Row(backBtn(rm)),
					)
					return c.Send(
						"🔗 *Ссылка для подключения VPN*\n\n"+
							"`"+remnaUser.SubscribeURL+"`\n\n"+
							"_Вставьте в Happ, V2RayN, Hiddify или другой совместимый клиент_",
						&tele.SendOptions{ParseMode: tele.ModeMarkdown},
						rm,
					)
				}

				// Path 3: create a new Remnawave account
				remnaUser, createErr := b.remna.CreateUser(ctx, remnaName, subs.ExpiresAt)
				if createErr != nil || remnaUser == nil || remnaUser.UUID == "" {
					b.log.Warn("remnawave lazy repair: create user failed", zap.Error(createErr))
					return c.Send("⚠️ Не удалось настроить VPN-аккаунт. Попробуйте позже.")
				}
				remnaUUID = remnaUser.UUID
				_ = b.repo.UpdateRemnaUUID(ctx, user.ID, remnaUUID)
				rm := &tele.ReplyMarkup{}
				btnCopy := rm.URL("📋 Открыть ссылку", remnaUser.SubscribeURL)
				rm.Inline(
					rm.Row(btnCopy),
					rm.Row(backBtn(rm)),
				)
				return c.Send(
					"🔗 *Ссылка для подключения VPN*\n\n"+
						"`"+remnaUser.SubscribeURL+"`\n\n"+
						"_Вставьте в Happ, V2RayN, Hiddify или другой совместимый клиент_",
					&tele.SendOptions{ParseMode: tele.ModeMarkdown},
					rm,
				)
			}
		}

		// Standard path: UUID exists, fetch from Remnawave
		remnaUser, err := b.remna.GetUser(ctx, remnaUUID)
		if err != nil {
			b.log.Warn("remnawave get user", zap.Error(err))
			return c.Send("⚠️ Не удалось загрузить данные подключения. Попробуйте позже.")
		}

		rm := &tele.ReplyMarkup{}
		btnCopy := rm.URL("📋 Открыть ссылку", remnaUser.SubscribeURL)
		rm.Inline(
			rm.Row(btnCopy),
			rm.Row(backBtn(rm)),
		)

		return c.Send(
			"🔗 *Ссылка для подключения VPN*\n\n"+
				"`"+remnaUser.SubscribeURL+"`\n\n"+
				"_Вставьте в Happ, V2RayN, Hiddify или другой совместимый клиент_",
			&tele.SendOptions{ParseMode: tele.ModeMarkdown},
			rm,
		)
	})
}

// ─── Unused import avoidance
var _ = platega.MethodSBPQR
