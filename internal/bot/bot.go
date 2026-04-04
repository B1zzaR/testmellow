// Package bot implements the Telegram bot interface for the VPN platform.
//
// Commands:
//
//	/start [referral_code] — register or login, process referral
//	/balance               — show YAD balance
//	/buy                   — buy subscription
//	/mysubs                — list subscriptions
//	/renew                 — renew subscription
//	/promo <code>          — apply promo code
//	/referral              — show referral link and stats
//	/ticket                — open/manage support tickets
//	/trial                 — activate free trial
package bot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"

	"github.com/vpnplatform/internal/domain"
	"github.com/vpnplatform/internal/integration/platega"
	"github.com/vpnplatform/internal/repository/postgres"
	"github.com/vpnplatform/internal/service"
	jwtpkg "github.com/vpnplatform/pkg/jwt"
)

type Bot struct {
	bot      *tele.Bot
	repo     *postgres.UserRepo
	auth     *service.AuthService
	subSvc   *service.SubscriptionService
	ecoSvc   *service.EconomyService
	trialSvc *service.TrialService
	jwtMgr   *jwtpkg.Manager
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
	jwtMgr *jwtpkg.Manager,
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
		jwtMgr:   jwtMgr,
		cfg:      cfg,
		log:      log,
	}

	bot.registerHandlers()
	return bot, nil
}

func (b *Bot) Start() {
	b.log.Info("telegram bot started")
	b.bot.Start()
}

func (b *Bot) Stop() {
	b.bot.Stop()
}

func (b *Bot) registerHandlers() {
	b.bot.Handle("/start", b.handleStart)
	b.bot.Handle("/balance", b.handleBalance)
	b.bot.Handle("/buy", b.handleBuy)
	b.bot.Handle("/mysubs", b.handleMySubs)
	b.bot.Handle("/renew", b.handleRenew)
	b.bot.Handle("/promo", b.handlePromo)
	b.bot.Handle("/referral", b.handleReferral)
	b.bot.Handle("/trial", b.handleTrial)
	b.bot.Handle("/ticket", b.handleTicketMenu)
	b.bot.Handle("/help", b.handleHelp)
}

// ─── /start ───────────────────────────────────────────────────────────────────

func (b *Bot) handleStart(c tele.Context) error {
	ctx := context.Background()
	tgID := c.Sender().ID
	username := c.Sender().Username

	// Check if user already exists
	user, err := b.repo.GetByTelegramID(ctx, int64(tgID))
	if err != nil {
		return c.Send("❌ Внутренняя ошибка. Попробуйте позже.")
	}

	if user != nil {
		if user.IsBanned {
			return c.Send("🚫 Ваш аккаунт заблокирован.")
		}
		return c.Send(fmt.Sprintf(
			"👋 С возвращением, %s!\n\n"+
				"💰 Баланс YAD: %d\n"+
				"🔗 Реферальный код: %s\n\n"+
				"Используйте /help для списка команд.",
			usernameOrID(username, int64(tgID)),
			user.YADBalance,
			user.ReferralCode,
		))
	}

	// New user — parse referral code from /start <referralCode>
	referralCode := ""
	parts := strings.Fields(c.Text())
	if len(parts) > 1 {
		referralCode = parts[1]
	}

	// Auto-register via Telegram with deterministic login
	login := fmt.Sprintf("tg%d", tgID)
	randPass := fmt.Sprintf("tg_%d_%d", tgID, time.Now().Unix())

	newUser, err := b.auth.Register(ctx, service.RegisterInput{
		Username:     login,
		Password:     randPass,
		ReferralCode: referralCode,
		IP:           "0.0.0.0", // no real IP available for bot
	})
	if err != nil {
		b.log.Warn("bot registration failed", zap.Error(err), zap.Int64("tg_id", int64(tgID)))
		return c.Send("❌ Не удалось зарегистрировать аккаунт. Попробуйте позже.")
	}

	// Link Telegram ID to the user
	tgIDVal := int64(tgID)
	newUser.TelegramID = &tgIDVal
	usernameVal := username
	newUser.Username = &usernameVal
	if err := b.repo.UpdateFingerprint(ctx, newUser.ID, "telegram", "0.0.0.0"); err != nil {
		b.log.Warn("update fingerprint failed", zap.Error(err))
	}

	welcomeMsg := fmt.Sprintf(
		"🎉 Добро пожаловать, %s!\n\n"+
			"✅ Аккаунт успешно создан.\n"+
			"🔗 Ваш реферальный код: %s\n\n"+
			"🎁 Используйте /trial для получения бесплатного пробного периода!\n"+
			"📋 /help — список всех команд",
		usernameOrID(username, int64(tgID)),
		newUser.ReferralCode,
	)
	return c.Send(welcomeMsg)
}

// ─── /balance ─────────────────────────────────────────────────────────────────

func (b *Bot) handleBalance(c tele.Context) error {
	ctx := context.Background()
	user, err := b.getUser(ctx, c)
	if err != nil {
		return c.Send("❌ " + err.Error())
	}

	return c.Send(fmt.Sprintf(
		"💰 *Ваш баланс YAD*\n\n"+
			"YAD: `%d`\n"+
			"В рублях: `%.2f ₽`\n\n"+
			"_1 YAD = 2.5 ₽_",
		user.YADBalance,
		float64(user.YADBalance)*2.5,
	), &tele.SendOptions{ParseMode: tele.ModeMarkdown})
}

// ─── /buy ─────────────────────────────────────────────────────────────────────

func (b *Bot) handleBuy(c tele.Context) error {
	ctx := context.Background()
	user, err := b.getUser(ctx, c)
	if err != nil {
		return c.Send("❌ " + err.Error())
	}

	selector := &tele.ReplyMarkup{}
	btnWeek := selector.Data("📅 1 неделя — 40 ₽", "buy_1week")
	btnMonth := selector.Data("📆 1 месяц — 100 ₽", "buy_1month")
	btnThree := selector.Data("🗓 3 месяца — 270 ₽", "buy_3months")

	selector.Inline(
		selector.Row(btnWeek),
		selector.Row(btnMonth),
		selector.Row(btnThree),
	)

	_ = user
	return c.Send("🛒 Выберите тарифный план:", selector)
}

func (b *Bot) handleBuyCallback(plan domain.SubscriptionPlan) tele.HandlerFunc {
	return func(c tele.Context) error {
		ctx := context.Background()
		user, err := b.getUser(ctx, c)
		if err != nil {
			return c.Respond(&tele.CallbackResponse{Text: "Ошибка: " + err.Error()})
		}

		redirect, payment, err := b.subSvc.InitiatePayment(ctx, user.ID, plan, b.cfg.PaymentReturnURL)
		if err != nil {
			return c.Respond(&tele.CallbackResponse{Text: "Ошибка: " + err.Error()})
		}

		msg := fmt.Sprintf(
			"💳 *Оплата подписки*\n\n"+
				"📋 Тариф: %s\n"+
				"💰 Сумма: %.2f ₽\n"+
				"🆔 ID платежа: `%s`\n\n"+
				"👆 [Перейти к оплате](%s)\n\n"+
				"_Ссылка действительна 15 минут_",
			plan,
			float64(payment.AmountKopecks)/100,
			payment.ID.String(),
			redirect,
		)
		return c.Send(msg, &tele.SendOptions{ParseMode: tele.ModeMarkdown})
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
	if err != nil || len(subs) == 0 {
		return c.Send("📭 У вас нет активных подписок.\n\nИспользуйте /buy для покупки.")
	}

	msg := "📋 *Ваши подписки:*\n\n"
	for _, sub := range subs {
		statusEmoji := "✅"
		if sub.Status == domain.SubStatusExpired {
			statusEmoji = "❌"
		} else if sub.Status == domain.SubStatusTrial {
			statusEmoji = "🆓"
		}
		msg += fmt.Sprintf(
			"%s *%s* (%s)\n   Истекает: `%s`\n\n",
			statusEmoji, sub.Plan, sub.Status,
			sub.ExpiresAt.Format("02.01.2006 15:04"),
		)
	}
	return c.Send(msg, &tele.SendOptions{ParseMode: tele.ModeMarkdown})
}

// ─── /renew ───────────────────────────────────────────────────────────────────

func (b *Bot) handleRenew(c tele.Context) error {
	// Same flow as /buy — initiate a new payment that extends the subscription
	return b.handleBuy(c)
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
		return c.Send("📢 Использование: /promo ВАШТЗАПРОС\nПример: /promo SUMMER2024")
	}
	code := strings.ToUpper(parts[1])

	promo, err := b.ecoSvc.UsePromoCode(ctx, user.ID, code)
	if err != nil {
		return c.Send("❌ " + err.Error())
	}

	return c.Send(fmt.Sprintf(
		"🎁 Промокод активирован!\n\n"+
			"💰 Начислено YAD: *%d*\n"+
			"_(эквивалент %.2f ₽)_",
		promo.YADAmount,
		float64(promo.YADAmount)*2.5,
	), &tele.SendOptions{ParseMode: tele.ModeMarkdown})
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

	return c.Send(fmt.Sprintf(
		"👥 *Реферальная программа*\n\n"+
			"🔗 Ваша ссылка:\n`%s`\n\n"+
			"👤 Приглашено пользователей: *%d*\n"+
			"💎 Вознаграждение: *15%%* от каждого платежа реферала\n\n"+
			"_Выплаты производятся в YAD с задержкой 24–72 ч.\n"+
			"30%% сразу, 70%% через 30 дней._",
		referralLink,
		len(refs),
	), &tele.SendOptions{ParseMode: tele.ModeMarkdown})
}

// ─── /trial ───────────────────────────────────────────────────────────────────

func (b *Bot) handleTrial(c tele.Context) error {
	ctx := context.Background()
	user, err := b.getUser(ctx, c)
	if err != nil {
		return c.Send("❌ " + err.Error())
	}

	sub, err := b.trialSvc.ActivateTrial(ctx, user.ID)
	if err != nil {
		return c.Send("❌ " + err.Error())
	}

	return c.Send(fmt.Sprintf(
		"🎉 *Пробный период активирован!*\n\n"+
			"⏰ Действует до: `%s`\n"+
			"📊 Статус: %s\n\n"+
			"_Для продления используйте /buy_",
		sub.ExpiresAt.Format("02.01.2006 15:04"),
		sub.Status,
	), &tele.SendOptions{ParseMode: tele.ModeMarkdown})
}

// ─── /ticket ──────────────────────────────────────────────────────────────────

func (b *Bot) handleTicketMenu(c tele.Context) error {
	ctx := context.Background()
	user, err := b.getUser(ctx, c)
	if err != nil {
		return c.Send("❌ " + err.Error())
	}

	tickets, _ := b.repo.ListTickets(ctx, &user.ID, "open", 5, 0)
	if len(tickets) == 0 {
		return c.Send(
			"🎫 *Поддержка*\n\n"+
				"У вас нет открытых тикетов.\n\n"+
				"Напишите сообщение, чтобы создать новый тикет:",
			&tele.SendOptions{ParseMode: tele.ModeMarkdown},
		)
	}

	msg := "🎫 *Ваши тикеты:*\n\n"
	for _, t := range tickets {
		msg += fmt.Sprintf("• `%s` — %s (%s)\n", t.ID.String()[:8], t.Subject, t.Status)
	}
	return c.Send(msg, &tele.SendOptions{ParseMode: tele.ModeMarkdown})
}

// ─── /help ────────────────────────────────────────────────────────────────────

func (b *Bot) handleHelp(c tele.Context) error {
	return c.Send(
		"🤖 *VPN Platform — Команды*\n\n"+
			"/start — Регистрация / вход\n"+
			"/trial — Бесплатный пробный период\n"+
			"/buy — Купить подписку\n"+
			"/renew — Продлить подписку\n"+
			"/mysubs — Мои подписки\n"+
			"/balance — Баланс YAD\n"+
			"/referral — Реферальная программа\n"+
			"/promo <код> — Применить промокод\n"+
			"/ticket — Поддержка\n"+
			"/help — Помощь",
		&tele.SendOptions{ParseMode: tele.ModeMarkdown},
	)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func (b *Bot) getUser(ctx context.Context, c tele.Context) (*domain.User, error) {
	tgID := int64(c.Sender().ID)
	user, err := b.repo.GetByTelegramID(ctx, tgID)
	if err != nil {
		return nil, fmt.Errorf("database error")
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

// SendNotification sends a message to a user by Telegram ID (called from worker)
func (b *Bot) SendNotification(tgID int64, message string) error {
	_, err := b.bot.Send(&tele.User{ID: tgID}, message)
	return err
}

// ─── Init buy callbacks (called after bot creation) ──────────────────────────

func (b *Bot) RegisterBuyCallbacks() {
	b.bot.Handle(&tele.Btn{Unique: "buy_1week"}, b.handleBuyCallback(domain.PlanWeek))
	b.bot.Handle(&tele.Btn{Unique: "buy_1month"}, b.handleBuyCallback(domain.PlanMonth))
	b.bot.Handle(&tele.Btn{Unique: "buy_3months"}, b.handleBuyCallback(domain.PlanThreeMonth))
}

// ─── Unused import avoidance
var _ = platega.MethodSBPQR
