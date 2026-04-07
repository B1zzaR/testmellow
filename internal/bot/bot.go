// Package bot implements the Telegram bot interface for the VPN platform.
//
// Commands:
//
//	/start [referral_code] — register or login, process referral
//	/balance               — show YAD balance + Telegram ID
//	/buy                   — buy subscription
//	/mysubs                — list subscriptions
//	/renew                 — renew subscription
//	/promo <code>          — apply promo code
//	/referral              — show referral link and stats
//	/ticket                — open/manage support tickets
//	/trial                 — info about free trial (redirect to site)
package bot

import (
	"context"
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
	"github.com/vpnplatform/internal/service"
	jwtpkg "github.com/vpnplatform/pkg/jwt"
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
	b.bot.Handle("/renew", b.handleBuy)
	b.bot.Handle("/promo", b.handlePromo)
	b.bot.Handle("/referral", b.handleReferral)
	b.bot.Handle("/trial", b.handleTrial)
	b.bot.Handle("/ticket", b.handleTicketMenu)
	b.bot.Handle("/help", b.handleHelp)
	b.bot.Handle("/link", b.handleLink)
}

// ─── Link handler ─────────────────────────────────────────────────────────────

// handleLink processes `/link CODE` — links the web account to this Telegram user.
func (b *Bot) handleLink(c tele.Context) error {
	args := strings.Fields(c.Message().Text)
	if len(args) < 2 {
		return c.Send("❌ Использование: /link КОД\nОткройте сайт → Настройки → «Привязать Telegram» и скопируйте команду.")
	}
	code := strings.ToUpper(strings.TrimSpace(args[1]))
	key := fmt.Sprintf("tg:link:%s", code)

	ctx := context.Background()
	userIDStr, err := b.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return c.Send("❌ Код не найден или истёк. Запросите новый код на сайте.")
	}
	if err != nil {
		b.log.Error("handleLink: redis get", zap.Error(err))
		return c.Send("⚠️ Временная ошибка, попробуйте позже.")
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		b.log.Error("handleLink: invalid user id in redis", zap.String("value", userIDStr))
		return c.Send("⚠️ Внутренняя ошибка, попробуйте запросить новый код.")
	}

	tgID := c.Sender().ID
	tgIDVal := int64(tgID)

	// Check if this telegram_id is already linked to some user
	existingUser, err := b.repo.GetByTelegramID(ctx, tgIDVal)
	if err != nil {
		b.log.Error("handleLink: get by telegram id", zap.Error(err))
		return c.Send("⚠️ Временная ошибка, попробуйте позже.")
	}

	// If telegram_id is linked to the same user, just confirm
	if existingUser != nil && existingUser.ID == userID {
		b.rdb.Del(ctx, key)
		return c.Send("✅ Этот Telegram уже привязан к вашему аккаунту!")
	}

	// If telegram_id is linked to another user, unlink it first
	if existingUser != nil && existingUser.ID != userID {
		if err := b.repo.SetTelegramID(ctx, existingUser.ID, nil); err != nil {
			b.log.Error("handleLink: unlink telegram from old user", zap.Error(err))
			return c.Send("⚠️ Не удалось выполнить операцию, попробуйте позже.")
		}
	}

	// Now link telegram_id to the target user
	if err := b.repo.SetTelegramID(ctx, userID, &tgIDVal); err != nil {
		b.log.Error("handleLink: set telegram id", zap.Error(err))
		return c.Send("⚠️ Не удалось привязать аккаунт, попробуйте снова.")
	}

	b.rdb.Del(ctx, key)
	return c.Send("✅ Telegram успешно привязан к вашему аккаунту!")
}

// ─── Main menu helpers ────────────────────────────────────────────────────────

func mainMenuText(user *domain.User, tgID int64, username string) string {
	name := username
	if name == "" {
		name = fmt.Sprintf("User%d", tgID)
	}
	adminLine := ""
	if user.IsAdmin {
		adminLine = "\n\n\u2699\ufe0f *admin mode*"
	}
	return fmt.Sprintf(
		"\U0001f40d _\u041d\u0435 \u043f\u0440\u044f\u0447\u044c\u0441\u044f \u0432 \u0442\u0435\u043d\u0438 \u2014 \u0441\u043a\u043e\u043b\u044c\u0437\u0438 \u043f\u043e \u0441\u0432\u0435\u0442\u0443!_\n\n"+
			"\u041f\u0440\u0438\u0432\u0435\u0442, *%s*\n"+
			"\u0411\u0435\u0437\u043b\u0438\u043c\u0438\u0442\u043d\u044b\u0439 VPN \u0437\u0430 \u043f\u0430\u0440\u0443 \u043a\u043b\u0438\u043a\u043e\u0432 \u26a1\n\n"+
			"\u0412\u0430\u0448 ID: `%d`\n"+
			"\U0001f3c6 \u041d\u0430\u043a\u043e\u043f\u043b\u0435\u043d\u043e \u044f\u0434\u0430: *%d*%s",
		name,
		tgID,
		user.YADBalance,
		adminLine,
	)
}

func (b *Bot) mainMenuMarkup(user *domain.User) *tele.ReplyMarkup {
	rm := &tele.ReplyMarkup{}

	btnTrial := rm.Data("\U0001f381 \u041f\u043e\u043f\u0440\u043e\u0431\u043e\u0432\u0430\u0442\u044c \u0431\u0435\u0441\u043f\u043b\u0430\u0442\u043d\u043e", "menu_trial")
	btnBuy := rm.Data("\U0001f48e \u041a\u0443\u043f\u0438\u0442\u044c \u043f\u043e\u0434\u043f\u0438\u0441\u043a\u0443", "menu_buy")
	btnPromo := rm.Data("\U0001f39f \u041f\u0440\u043e\u043c\u043e\u043a\u043e\u0434", "menu_promo")
	btnSupport := rm.Data("\U0001f527 \u041f\u043e\u0434\u0434\u0435\u0440\u0436\u043a\u0430", "menu_support")
	btnMySubs := rm.Data("\U0001f4c1 \u041c\u043e\u0438 \u043f\u043e\u0434\u043f\u0438\u0441\u043a\u0438", "menu_mysubs")
	btnHelp := rm.Data("\U0001f4ac \u041f\u043e\u043c\u043e\u0449\u044c", "menu_help")
	btnReferrals := rm.Data("\U0001f464 \u0420\u0435\u0444\u0435\u0440\u0430\u043b\u044b", "menu_referrals")
	btnYadShop := rm.Data("\U0001f6d2 \u041c\u0430\u0433\u0430\u0437\u0438\u043d \u044f\u0434\u0430", "menu_yadshop")

	rows := []tele.Row{
		rm.Row(btnTrial),
		rm.Row(btnBuy),
		rm.Row(btnPromo, btnSupport),
		rm.Row(btnMySubs, btnHelp),
		rm.Row(btnReferrals, btnYadShop),
	}
	if user.IsAdmin {
		btnAdmin := rm.Data("\u2699\ufe0f \u041f\u0430\u043d\u0435\u043b\u044c", "menu_admin")
		rows = append(rows, rm.Row(btnAdmin))
	}
	rm.Inline(rows...)
	return rm
}

func backBtn(rm *tele.ReplyMarkup) tele.Btn {
	return rm.Data("\U0001f3e0 \u0413\u043b\u0430\u0432\u043d\u043e\u0435 \u043c\u0435\u043d\u044e", "menu_back")
}

func (b *Bot) sendMainMenu(c tele.Context) error {
	ctx := context.Background()
	user, err := b.getUser(ctx, c)
	if err != nil {
		return c.Send("\u274c " + err.Error())
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
		return c.Send("\u274c \u0412\u043d\u0443\u0442\u0440\u0435\u043d\u043d\u044f\u044f \u043e\u0448\u0438\u0431\u043a\u0430. \u041f\u043e\u043f\u0440\u043e\u0431\u0443\u0439\u0442\u0435 \u043f\u043e\u0437\u0436\u0435.")
	}

	if user != nil {
		if user.IsBanned {
			return c.Send("\U0001f6ab \u0412\u0430\u0448 \u0430\u043a\u043a\u0430\u0443\u043d\u0442 \u0437\u0430\u0431\u043b\u043e\u043a\u0438\u0440\u043e\u0432\u0430\u043d.")
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
	randPass := fmt.Sprintf("tg_%d_%d", tgID, time.Now().Unix())

	newUser, err := b.auth.Register(ctx, service.RegisterInput{
		Username:     login,
		Password:     randPass,
		ReferralCode: referralCode,
		IP:           "0.0.0.0",
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
		return c.Send("\u274c \u041d\u0435 \u0443\u0434\u0430\u043b\u043e\u0441\u044c \u0437\u0430\u0440\u0435\u0433\u0438\u0441\u0442\u0440\u0438\u0440\u043e\u0432\u0430\u0442\u044c \u0430\u043a\u043a\u0430\u0443\u043d\u0442. \u041f\u043e\u043f\u0440\u043e\u0431\u0443\u0439\u0442\u0435 \u043f\u043e\u0437\u0436\u0435.")
	}

	tgIDVal := int64(tgID)
	if err := b.repo.SetTelegramID(ctx, newUser.ID, &tgIDVal); err != nil {
		b.log.Warn("set telegram id after registration", zap.Error(err))
	}

	name := username
	if name == "" {
		name = fmt.Sprintf("User%d", tgID)
	}
	_ = c.Send(
		fmt.Sprintf("\U0001f389 *\u0414\u043e\u0431\u0440\u043e \u043f\u043e\u0436\u0430\u043b\u043e\u0432\u0430\u0442\u044c, %s!*\n\n\u0410\u043a\u043a\u0430\u0443\u043d\u0442 \u0443\u0441\u043f\u0435\u0448\u043d\u043e \u0441\u043e\u0437\u0434\u0430\u043d \u2705", name),
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
		return c.Send("\u274c " + err.Error())
	}

	tgID := c.Sender().ID
	rm := &tele.ReplyMarkup{}
	btnBuyInline := rm.Data("\U0001f48e \u041a\u0443\u043f\u0438\u0442\u044c \u043f\u043e\u0434\u043f\u0438\u0441\u043a\u0443", "menu_buy")
	btnYad := rm.Data("\U0001f6d2 \u041c\u0430\u0433\u0430\u0437\u0438\u043d \u044f\u0434\u0430", "menu_yadshop")
	rm.Inline(
		rm.Row(btnBuyInline, btnYad),
		rm.Row(backBtn(rm)),
	)

	return c.Send(fmt.Sprintf(
		"\U0001f4b0 *\u0412\u0430\u0448 \u043a\u043e\u0448\u0435\u043b\u0451\u043a*\n"+
			"\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\n"+
			"\U0001f194 Telegram ID: `%d`\n"+
			"\U0001f3c6 \u042f\u0414: *%d*\n"+
			"\U0001f4b5 \u0412 \u0440\u0443\u0431\u043b\u044f\u0445: *%.0f \u20bd*\n\n"+
			"\U0001f4ca \u041a\u0443\u0440\u0441: 1 \u042f\u0414 = 2.5 \u20bd",
		tgID,
		user.YADBalance,
		float64(user.YADBalance)*2.5,
	), &tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm)
}

// ─── /buy ─────────────────────────────────────────────────────────────────────

func (b *Bot) handleBuy(c tele.Context) error {
	ctx := context.Background()
	_, err := b.getUser(ctx, c)
	if err != nil {
		return c.Send("\u274c " + err.Error())
	}

	rm := &tele.ReplyMarkup{}
	btnWeek := rm.Data("\U0001f4c5 1 \u043d\u0435\u0434\u0435\u043b\u044f \u2014 40 \u20bd", "buy_1week")
	btnMonth := rm.Data("\U0001f4c6 1 \u043c\u0435\u0441\u044f\u0446 \u2014 100 \u20bd", "buy_1month")
	btnThree := rm.Data("\U0001f5d3 3 \u043c\u0435\u0441\u044f\u0446\u0430 \u2014 270 \u20bd \U0001f525", "buy_3months")
	rm.Inline(
		rm.Row(btnWeek),
		rm.Row(btnMonth),
		rm.Row(btnThree),
		rm.Row(backBtn(rm)),
	)

	return c.Send(
		"\U0001f48e *\u041a\u0443\u043f\u0438\u0442\u044c VPN \u043f\u043e\u0434\u043f\u0438\u0441\u043a\u0443*\n"+
			"\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\n\n"+
			"\U0001f4c5 1 \u043d\u0435\u0434\u0435\u043b\u044f \u2014 *40 \u20bd* (5.7 \u20bd/\u0434\u0435\u043d\u044c)\n"+
			"\U0001f4c6 1 \u043c\u0435\u0441\u044f\u0446 \u2014 *100 \u20bd* (3.3 \u20bd/\u0434\u0435\u043d\u044c)\n"+
			"\U0001f5d3 3 \u043c\u0435\u0441\u044f\u0446\u0430 \u2014 *270 \u20bd* (3.0 \u20bd/\u0434\u0435\u043d\u044c) \U0001f525\n\n"+
			"\u0412\u044b\u0431\u0435\u0440\u0438\u0442\u0435 \u0442\u0430\u0440\u0438\u0444:",
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

		msgText := fmt.Sprintf(
			"💳 *Оплата подписки*\n"+
				"─────────────────────\n"+
				"📋 Тариф: *%s*\n"+
				"💰 Сумма: *%.0f ₽*\n"+
				"🆔 ID платежа: `%s`\n\n"+
				"_Ссылка действительна 15 минут_",
			planName(plan),
			float64(payment.AmountKopecks)/100,
			payment.ID.String(),
		)

		return c.Send(msgText, &tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm)
	}
}

// ─── /mysubs ──────────────────────────────────────────────────────────────────

func (b *Bot) handleMySubs(c tele.Context) error {
	ctx := context.Background()
	user, err := b.getUser(ctx, c)
	if err != nil {
		return c.Send("\u274c " + err.Error())
	}

	subs, err := b.subSvc.GetUserSubscriptions(ctx, user.ID)

	rm := &tele.ReplyMarkup{}
	if err != nil || len(subs) == 0 {
		btnBuyInline := rm.Data("\U0001f48e \u041a\u0443\u043f\u0438\u0442\u044c \u043f\u043e\u0434\u043f\u0438\u0441\u043a\u0443", "menu_buy")
		rm.Inline(
			rm.Row(btnBuyInline),
			rm.Row(backBtn(rm)),
		)
		return c.Send(
			"\U0001f4c1 *\u041c\u043e\u0438 \u043f\u043e\u0434\u043f\u0438\u0441\u043a\u0438*\n\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\n\n"+
				"\U0001f4ed \u0423 \u0432\u0430\u0441 \u043d\u0435\u0442 \u0430\u043a\u0442\u0438\u0432\u043d\u044b\u0445 \u043f\u043e\u0434\u043f\u0438\u0441\u043e\u043a.",
			&tele.SendOptions{ParseMode: tele.ModeMarkdown},
			rm,
		)
	}

	msg := "\U0001f4c1 *\u041c\u043e\u0438 \u043f\u043e\u0434\u043f\u0438\u0441\u043a\u0438*\n\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501"
	for i, sub := range subs {
		statusEmoji := "\u2705"
		statusText := "\u0410\u043a\u0442\u0438\u0432\u043d\u0430"
		switch sub.Status {
		case domain.SubStatusExpired:
			statusEmoji = "\u274c"
			statusText = "\u0418\u0441\u0442\u0435\u043a\u043b\u0430"
		case domain.SubStatusTrial:
			statusEmoji = "\U0001f193"
			statusText = "\u041f\u0440\u043e\u0431\u043d\u0430\u044f"
		case domain.SubStatusCanceled:
			statusEmoji = "\U0001f6ab"
			statusText = "\u041e\u0442\u043c\u0435\u043d\u0435\u043d\u0430"
		}
		daysLeft := int(time.Until(sub.ExpiresAt).Hours() / 24)
		daysStr := ""
		if (sub.Status == domain.SubStatusActive || sub.Status == domain.SubStatusTrial) && daysLeft >= 0 {
			daysStr = fmt.Sprintf(" \u00b7 *%d \u0434\u043d.*", daysLeft)
		}
		msg += fmt.Sprintf(
			"\n\n%d. %s *%s* \u2014 %s%s\n   \U0001f4c5 \u0434\u043e `%s`",
			i+1, statusEmoji, planName(sub.Plan), statusText, daysStr,
			sub.ExpiresAt.Format("02.01.2006"),
		)
	}

	btnRenew := rm.Data("\U0001f504 \u041f\u0440\u043e\u0434\u043b\u0438\u0442\u044c \u043f\u043e\u0434\u043f\u0438\u0441\u043a\u0443", "menu_buy")
	btnSubscribeLink := rm.Data("\U0001f517 \u041a\u043e\u043f\u0438\u0440\u043e\u0432\u0430\u0442\u044c \u0441\u0441\u044b\u043b\u043a\u0443 VPN", "get_vpn_link")
	rm.Inline(
		rm.Row(btnRenew, btnSubscribeLink),
		rm.Row(backBtn(rm)),
	)
	return c.Send(msg, &tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm)
}

// ─── /promo ───────────────────────────────────────────────────────────────────

func (b *Bot) handlePromo(c tele.Context) error {
	ctx := context.Background()
	user, err := b.getUser(ctx, c)
	if err != nil {
		return c.Send("\u274c " + err.Error())
	}

	parts := strings.Fields(c.Text())
	if len(parts) < 2 {
		rm := &tele.ReplyMarkup{}
		rm.Inline(rm.Row(backBtn(rm)))
		return c.Send(
			"\U0001f39f *\u0410\u043a\u0442\u0438\u0432\u0430\u0446\u0438\u044f \u043f\u0440\u043e\u043c\u043e\u043a\u043e\u0434\u0430*\n\n"+
				"\u041e\u0442\u043f\u0440\u0430\u0432\u044c\u0442\u0435 \u043a\u043e\u043c\u0430\u043d\u0434\u0443 \u0432 \u0444\u043e\u0440\u043c\u0430\u0442\u0435:\n"+
				"`/promo \u041a\u041e\u0414`\n\n\u041d\u0430\u043f\u0440\u0438\u043c\u0435\u0440: `/promo SUMMER2024`",
			&tele.SendOptions{ParseMode: tele.ModeMarkdown},
			rm,
		)
	}
	code := strings.ToUpper(parts[1])

	promo, err := b.ecoSvc.UsePromoCode(ctx, user.ID, code)
	if err != nil {
		return c.Send("\u274c " + err.Error())
	}

	rm := &tele.ReplyMarkup{}
	rm.Inline(rm.Row(backBtn(rm)))
	return c.Send(fmt.Sprintf(
		"\U0001f39f *\u041f\u0440\u043e\u043c\u043e\u043a\u043e\u0434 \u0430\u043a\u0442\u0438\u0432\u0438\u0440\u043e\u0432\u0430\u043d!*\n\n"+
			"\U0001f3c6 \u041d\u0430\u0447\u0438\u0441\u043b\u0435\u043d\u043e \u042f\u0414: *%d*\n"+
			"\U0001f4b5 \u042d\u043a\u0432\u0438\u0432\u0430\u043b\u0435\u043d\u0442: *%.0f \u20bd*",
		promo.YADAmount,
		float64(promo.YADAmount)*2.5,
	), &tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm)
}

// ─── /referral ────────────────────────────────────────────────────────────────

func (b *Bot) handleReferral(c tele.Context) error {
	ctx := context.Background()
	user, err := b.getUser(ctx, c)
	if err != nil {
		return c.Send("\u274c " + err.Error())
	}

	refs, _ := b.repo.GetReferralsByReferrer(ctx, user.ID)
	referralLink := fmt.Sprintf("https://t.me/%s?start=%s", b.bot.Me.Username, user.ReferralCode)

	shareURL := fmt.Sprintf(
		"https://t.me/share/url?url=%s&text=VPN%%20\u043f\u043b\u0430\u0442\u0444\u043e\u0440\u043c\u0430%%20\u2014%%20\u0431\u044b\u0441\u0442\u0440\u044b\u0439%%20\u0438%%20\u0431\u0435\u0437\u043e\u043f\u0430\u0441\u043d\u044b\u0439%%20VPN",
		referralLink,
	)

	rm := &tele.ReplyMarkup{}
	btnShare := rm.URL("\U0001f4e4 \u041f\u043e\u0434\u0435\u043b\u0438\u0442\u044c\u0441\u044f \u0441\u0441\u044b\u043b\u043a\u043e\u0439", shareURL)
	rm.Inline(
		rm.Row(btnShare),
		rm.Row(backBtn(rm)),
	)

	return c.Send(fmt.Sprintf(
		"\U0001f464 *\u0420\u0435\u0444\u0435\u0440\u0430\u043b\u044c\u043d\u0430\u044f \u043f\u0440\u043e\u0433\u0440\u0430\u043c\u043c\u0430*\n"+
			"\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\n"+
			"\U0001f517 \u0412\u0430\u0448\u0430 \u0441\u0441\u044b\u043b\u043a\u0430:\n`%s`\n\n"+
			"\U0001f465 \u041f\u0440\u0438\u0433\u043b\u0430\u0448\u0435\u043d\u043e: *%d* \u0447\u0435\u043b.\n"+
			"\U0001f48e \u0412\u043e\u0437\u043d\u0430\u0433\u0440\u0430\u0436\u0434\u0435\u043d\u0438\u0435: *15%%* \u043e\u0442 \u043a\u0430\u0436\u0434\u043e\u0433\u043e \u043f\u043b\u0430\u0442\u0435\u0436\u0430\n\n"+
			"\U0001f4a1 *\u041a\u0430\u043a \u044d\u0442\u043e \u0440\u0430\u0431\u043e\u0442\u0430\u0435\u0442:*\n"+
			"\u2022 30%% \u0432\u044b\u043f\u043b\u0430\u0447\u0438\u0432\u0430\u0435\u0442\u0441\u044f \u0441\u0440\u0430\u0437\u0443\n"+
			"\u2022 70%% \u0447\u0435\u0440\u0435\u0437 30 \u0434\u043d\u0435\u0439\n"+
			"\u2022 \u0412\u044b\u043f\u043b\u0430\u0442\u044b \u0432 \u042f\u0414",
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
	btnRegister := rm.URL("\U0001f310 \u0417\u0430\u0440\u0435\u0433\u0438\u0441\u0442\u0440\u0438\u0440\u043e\u0432\u0430\u0442\u044c\u0441\u044f \u043d\u0430 \u0441\u0430\u0439\u0442\u0435", webURL+"/register")
	rm.Inline(
		rm.Row(btnRegister),
		rm.Row(backBtn(rm)),
	)

	return c.Send(
		"\U0001f381 *\u041f\u0440\u043e\u0431\u043d\u044b\u0439 \u043f\u0435\u0440\u0438\u043e\u0434*\n"+
			"\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\n\n"+
			"\u041f\u0440\u043e\u0431\u043d\u0430\u044f \u043f\u043e\u0434\u043f\u0438\u0441\u043a\u0430 \u0434\u043e\u0441\u0442\u0443\u043f\u043d\u0430 \u0442\u043e\u043b\u044c\u043a\u043e \u0447\u0435\u0440\u0435\u0437 \u0441\u0430\u0439\u0442.\n\n"+
			"\U0001f4cb *\u041a\u0430\u043a \u043f\u043e\u043b\u0443\u0447\u0438\u0442\u044c:*\n"+
			"1\ufe0f\u20e3 \u0417\u0430\u0440\u0435\u0433\u0438\u0441\u0442\u0440\u0438\u0440\u0443\u0439\u0442\u0435\u0441\u044c \u043d\u0430 \u0441\u0430\u0439\u0442\u0435\n"+
			"2\ufe0f\u20e3 \u041f\u0435\u0440\u0435\u0439\u0434\u0438\u0442\u0435 \u0432 \u0440\u0430\u0437\u0434\u0435\u043b *\u00ab\u041f\u043e\u0434\u043f\u0438\u0441\u043a\u0438\u00bb*\n"+
			"3\ufe0f\u20e3 \u041d\u0430\u0436\u043c\u0438\u0442\u0435 *\u00ab\u041f\u043e\u043b\u0443\u0447\u0438\u0442\u044c \u043f\u0440\u043e\u0431\u043d\u044b\u0439 \u043f\u0435\u0440\u0438\u043e\u0434\u00bb*\n\n"+
			"_\u041f\u043e\u0441\u043b\u0435 \u0440\u0435\u0433\u0438\u0441\u0442\u0440\u0430\u0446\u0438\u0438 \u043f\u0440\u0438\u0432\u044f\u0436\u0438\u0442\u0435 Telegram \u0432 \u043d\u0430\u0441\u0442\u0440\u043e\u0439\u043a\u0430\u0445_",
		&tele.SendOptions{ParseMode: tele.ModeMarkdown},
		rm,
	)
}

// ─── /ticket ──────────────────────────────────────────────────────────────────

func (b *Bot) handleTicketMenu(c tele.Context) error {
	ctx := context.Background()
	user, err := b.getUser(ctx, c)
	if err != nil {
		return c.Send("\u274c " + err.Error())
	}

	tickets, _ := b.repo.ListTickets(ctx, &user.ID, "open", 5, 0)
	rm := &tele.ReplyMarkup{}
	rm.Inline(rm.Row(backBtn(rm)))

	if len(tickets) == 0 {
		return c.Send(
			"\U0001f527 *\u041f\u043e\u0434\u0434\u0435\u0440\u0436\u043a\u0430*\n\n"+
				"\u0423 \u0432\u0430\u0441 \u043d\u0435\u0442 \u043e\u0442\u043a\u0440\u044b\u0442\u044b\u0445 \u0442\u0438\u043a\u0435\u0442\u043e\u0432.\n\n"+
				"\u0414\u043b\u044f \u0441\u043e\u0437\u0434\u0430\u043d\u0438\u044f \u043e\u0431\u0440\u0430\u0442\u0438\u0442\u0435\u0441\u044c \u043d\u0430 \u0441\u0430\u0439\u0442.",
			&tele.SendOptions{ParseMode: tele.ModeMarkdown},
			rm,
		)
	}

	msg := "\U0001f527 *\u0412\u0430\u0448\u0438 \u0442\u0438\u043a\u0435\u0442\u044b:*\n\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\n"
	for _, t := range tickets {
		statusEmoji := "\U0001f7e2"
		if t.Status == "closed" {
			statusEmoji = "\U0001f534"
		} else if t.Status == "answered" {
			statusEmoji = "\U0001f7e1"
		}
		msg += fmt.Sprintf("%s `%s` \u2014 %s\n", statusEmoji, t.ID.String()[:8], t.Subject)
	}
	return c.Send(msg, &tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm)
}

// ─── /help ────────────────────────────────────────────────────────────────────

func (b *Bot) handleHelp(c tele.Context) error {
	rm := &tele.ReplyMarkup{}
	rm.Inline(rm.Row(backBtn(rm)))
	return c.Send(
		"\U0001f4ac *\u041f\u043e\u043c\u043e\u0449\u044c*\n"+
			"\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\n\n"+
			"\U0001f4b0 /balance \u2014 \u0411\u0430\u043b\u0430\u043d\u0441 \u0438 Telegram ID\n"+
			"\U0001f4c1 /mysubs \u2014 \u041c\u043e\u0438 \u043f\u043e\u0434\u043f\u0438\u0441\u043a\u0438\n"+
			"\U0001f48e /buy \u2014 \u041a\u0443\u043f\u0438\u0442\u044c \u043f\u043e\u0434\u043f\u0438\u0441\u043a\u0443\n"+
			"\U0001f381 /trial \u2014 \u041f\u0440\u043e\u0431\u043d\u044b\u0439 \u043f\u0435\u0440\u0438\u043e\u0434\n"+
			"\U0001f464 /referral \u2014 \u0420\u0435\u0444\u0435\u0440\u0430\u043b\u044c\u043d\u0430\u044f \u043f\u0440\u043e\u0433\u0440\u0430\u043c\u043c\u0430\n"+
			"\U0001f527 /ticket \u2014 \u041f\u043e\u0434\u0434\u0435\u0440\u0436\u043a\u0430\n"+
			"\U0001f39f /promo <\u043a\u043e\u0434> \u2014 \u0410\u043a\u0442\u0438\u0432\u0438\u0440\u043e\u0432\u0430\u0442\u044c \u043f\u0440\u043e\u043c\u043e\u043a\u043e\u0434\n\n"+
			"_\u041f\u043e \u0432\u0441\u0435\u043c \u0432\u043e\u043f\u0440\u043e\u0441\u0430\u043c \u043e\u0431\u0440\u0430\u0449\u0430\u0439\u0442\u0435\u0441\u044c \u0447\u0435\u0440\u0435\u0437 /ticket_",
		&tele.SendOptions{ParseMode: tele.ModeMarkdown},
		rm,
	)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func (b *Bot) getUser(ctx context.Context, c tele.Context) (*domain.User, error) {
	tgID := int64(c.Sender().ID)
	user, err := b.repo.GetByTelegramID(ctx, tgID)
	if err != nil {
		return nil, fmt.Errorf("\u043e\u0448\u0438\u0431\u043a\u0430 \u0431\u0430\u0437\u044b \u0434\u0430\u043d\u043d\u044b\u0445")
	}
	if user == nil {
		return nil, fmt.Errorf("\u0430\u043a\u043a\u0430\u0443\u043d\u0442 \u043d\u0435 \u043d\u0430\u0439\u0434\u0435\u043d. \u0418\u0441\u043f\u043e\u043b\u044c\u0437\u0443\u0439\u0442\u0435 /start \u0434\u043b\u044f \u0440\u0435\u0433\u0438\u0441\u0442\u0440\u0430\u0446\u0438\u0438")
	}
	if user.IsBanned {
		return nil, fmt.Errorf("\u0432\u0430\u0448 \u0430\u043a\u043a\u0430\u0443\u043d\u0442 \u0437\u0430\u0431\u043b\u043e\u043a\u0438\u0440\u043e\u0432\u0430\u043d")
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
		return "1 \u043d\u0435\u0434\u0435\u043b\u044f"
	case domain.PlanMonth:
		return "1 \u043c\u0435\u0441\u044f\u0446"
	case domain.PlanThreeMonth:
		return "3 \u043c\u0435\u0441\u044f\u0446\u0430"
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
			"\U0001f39f *\u041f\u0440\u043e\u043c\u043e\u043a\u043e\u0434*\n\n\u041e\u0442\u043f\u0440\u0430\u0432\u044c\u0442\u0435 \u043a\u043e\u043c\u0430\u043d\u0434\u0443:\n`/promo \u041a\u041e\u0414`\n\n\u041d\u0430\u043f\u0440\u0438\u043c\u0435\u0440: `/promo SUMMER2024`",
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
	b.bot.Handle(&tele.Btn{Unique: "menu_yadshop"}, func(c tele.Context) error {
		_ = c.Respond()
		ctx := context.Background()
		user, err := b.getUser(ctx, c)
		if err != nil {
			return c.Send("\u274c " + err.Error())
		}
		webURL := b.cfg.WebAppURL
		if webURL == "" {
			webURL = "https://vpn-platform.ru"
		}
		rm := &tele.ReplyMarkup{}
		btnShop := rm.URL("\U0001f6d2 \u041e\u0442\u043a\u0440\u044b\u0442\u044c \u043c\u0430\u0433\u0430\u0437\u0438\u043d", webURL+"/shop")
		rm.Inline(
			rm.Row(btnShop),
			rm.Row(backBtn(rm)),
		)
		return c.Send(fmt.Sprintf(
			"\U0001f6d2 *\u041c\u0430\u0433\u0430\u0437\u0438\u043d \u044f\u0434\u0430*\n"+
				"\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\n"+
				"\U0001f3c6 \u0412\u0430\u0448 \u0431\u0430\u043b\u0430\u043d\u0441: *%d \u042f\u0414* (%.0f \u20bd)\n\n"+
				"\u041f\u043e\u043a\u0443\u043f\u0430\u0439\u0442\u0435 \u043f\u043e\u0434\u043f\u0438\u0441\u043a\u0438 \u0437\u0430 \u042f\u0414 \u0432\u044b\u0433\u043e\u0434\u043d\u0435\u0435!\n\n"+
				"\U0001f4c5 1 \u043d\u0435\u0434\u0435\u043b\u044f \u2014 *%d \u042f\u0414*\n"+
				"\U0001f4c6 1 \u043c\u0435\u0441\u044f\u0446 \u2014 *%d \u042f\u0414*\n"+
				"\U0001f5d3 3 \u043c\u0435\u0441\u044f\u0446\u0430 \u2014 *%d \u042f\u0414* \U0001f525\n\n"+
				"_\u0414\u043b\u044f \u043f\u043e\u043a\u0443\u043f\u043a\u0438 \u043f\u0435\u0440\u0435\u0439\u0434\u0438\u0442\u0435 \u043d\u0430 \u0441\u0430\u0439\u0442:_",
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
			return c.Respond(&tele.CallbackResponse{Text: "\u0414\u043e\u0441\u0442\u0443\u043f \u0437\u0430\u043f\u0440\u0435\u0449\u0451\u043d"})
		}
		webURL := b.cfg.WebAppURL
		if webURL == "" {
			webURL = "https://vpn-platform.ru"
		}
		rm := &tele.ReplyMarkup{}
		btnPanel := rm.URL("\U0001f5a5 \u041e\u0442\u043a\u0440\u044b\u0442\u044c \u043f\u0430\u043d\u0435\u043b\u044c", webURL+"/admin")
		rm.Inline(
			rm.Row(btnPanel),
			rm.Row(backBtn(rm)),
		)
		return c.Send(
			"\u2699\ufe0f *\u041f\u0430\u043d\u0435\u043b\u044c \u0430\u0434\u043c\u0438\u043d\u0438\u0441\u0442\u0440\u0430\u0442\u043e\u0440\u0430*\n\n\u041e\u0442\u043a\u0440\u043e\u0439\u0442\u0435 \u0432\u0435\u0431-\u043f\u0430\u043d\u0435\u043b\u044c \u0434\u043b\u044f \u0443\u043f\u0440\u0430\u0432\u043b\u0435\u043d\u0438\u044f.",
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
				return c.Send("❌ Активная подписка не найдена")
			}

			// Path 1: subscription has a stored remna_sub_uuid — use it directly
			if subs.RemnaSubUUID != nil && *subs.RemnaSubUUID != "" {
				remnaUUID = *subs.RemnaSubUUID
				_ = b.repo.UpdateRemnaUUID(ctx, user.ID, remnaUUID)
			} else {
				// Path 2: look up by username in Remnawave
				remnaUser, lookupErr := b.remna.GetUserByUsername(ctx, user.ID.String())
				if lookupErr == nil && remnaUser != nil && remnaUser.UUID != "" {
					remnaUUID = remnaUser.UUID
					_ = b.repo.UpdateRemnaUUID(ctx, user.ID, remnaUUID)
					rm := &tele.ReplyMarkup{}
					btnCopy := rm.URL("📋 Скопировать", remnaUser.SubscribeURL)
					rm.Inline(
						rm.Row(btnCopy),
						rm.Row(backBtn(rm)),
					)
					return c.Send(
						"🔗 *Ссылка для подключения VPN:*\n\n"+
							"`"+remnaUser.SubscribeURL+"`\n\n"+
							"_Вставьте в Happ, V2RayN, Hiddify или любой совместимый клиент VPN_",
						&tele.SendOptions{ParseMode: tele.ModeMarkdown},
						rm,
					)
				}

				// Path 3: create a new Remnawave account
				remnaUser, createErr := b.remna.CreateUser(ctx, user.ID.String(), subs.ExpiresAt)
				if createErr != nil || remnaUser == nil || remnaUser.UUID == "" {
					b.log.Warn("remnawave lazy repair: create user failed", zap.Error(createErr))
					return c.Send("⚠️ Не удалось настроить VPN-аккаунт, попробуйте позже")
				}
				remnaUUID = remnaUser.UUID
				_ = b.repo.UpdateRemnaUUID(ctx, user.ID, remnaUUID)
				rm := &tele.ReplyMarkup{}
				btnCopy := rm.URL("📋 Скопировать", remnaUser.SubscribeURL)
				rm.Inline(
					rm.Row(btnCopy),
					rm.Row(backBtn(rm)),
				)
				return c.Send(
					"🔗 *Ссылка для подключения VPN:*\n\n"+
						"`"+remnaUser.SubscribeURL+"`\n\n"+
						"_Вставьте в Happ, V2RayN, Hiddify или любой совместимый клиент VPN_",
					&tele.SendOptions{ParseMode: tele.ModeMarkdown},
					rm,
				)
			}
		}

		// Standard path: UUID exists, fetch from Remnawave
		remnaUser, err := b.remna.GetUser(ctx, remnaUUID)
		if err != nil {
			b.log.Warn("remnawave get user", zap.Error(err))
			return c.Send("⚠️ Не удалось загрузить данные подключения")
		}

		rm := &tele.ReplyMarkup{}
		btnCopy := rm.URL("📋 Скопировать", remnaUser.SubscribeURL)
		rm.Inline(
			rm.Row(btnCopy),
			rm.Row(backBtn(rm)),
		)

		return c.Send(
			"🔗 *Ссылка для подключения VPN:*\n\n"+
				"`"+remnaUser.SubscribeURL+"`\n\n"+
				"_Вставьте в Happ, V2RayN, Hiddify или любой совместимый клиент VPN_",
			&tele.SendOptions{ParseMode: tele.ModeMarkdown},
			rm,
		)
	})
}

// ─── Unused import avoidance
var _ = platega.MethodSBPQR
