// Package bot implements the Telegram bot interface for the VPN platform.
package bot

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
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
	devSvc   *service.DeviceService
	remna    *remnawave.Client
	jwtMgr   *jwtpkg.Manager
	rdb      *redis.Client
	cfg      BotConfig
	log      *zap.Logger
}

type BotConfig struct {
	Token            string
	AdminID          int64
	WebAppURL        string
	PaymentReturnURL string
}

func New(
	cfg BotConfig,
	repo *postgres.UserRepo,
	auth *service.AuthService,
	subSvc *service.SubscriptionService,
	ecoSvc *service.EconomyService,
	trialSvc *service.TrialService,
	devSvc *service.DeviceService,
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
		devSvc:   devSvc,
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

// ─── Queue system ─────────────────────────────────────────────────────────────

func (b *Bot) StartQueues(ctx context.Context) {
	go b.queueLoop(ctx, "queue:notify:telegram", b.handleNotifyTelegramQueue)
	go b.queueLoop(ctx, "queue:tfa:challenge", b.handleTFAChallengeQueue)
	<-ctx.Done()
}

func (b *Bot) queueLoop(ctx context.Context, queue string, handler func(ctx context.Context, payload string) error) {
	b.log.Info("bot queue loop started", zap.String("queue", queue))
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		result, err := b.rdb.BRPop(ctx, 5*time.Second, queue).Result()
		if err != nil {
			if err != redis.Nil && ctx.Err() == nil {
				b.log.Error("bot brpop error", zap.String("queue", queue), zap.Error(err))
				time.Sleep(time.Second)
			}
			continue
		}
		if len(result) < 2 {
			continue
		}
		if err := handler(ctx, result[1]); err != nil {
			b.log.Error("bot queue handler error", zap.String("queue", queue), zap.Error(err))
		}
	}
}

func (b *Bot) handleNotifyTelegramQueue(_ context.Context, payload string) error {
	var job struct {
		TelegramID int64  `json:"telegram_id"`
		Message    string `json:"message"`
	}
	if err := json.Unmarshal([]byte(payload), &job); err != nil {
		return err
	}
	_, err := b.bot.Send(&tele.User{ID: job.TelegramID}, job.Message, &tele.SendOptions{ParseMode: tele.ModeHTML})
	return err
}

func (b *Bot) handleTFAChallengeQueue(_ context.Context, payload string) error {
	var job struct {
		TelegramID  int64  `json:"telegram_id"`
		ChallengeID string `json:"challenge_id"`
		Message     string `json:"message"`
	}
	if err := json.Unmarshal([]byte(payload), &job); err != nil {
		return err
	}
	approveBtn := tele.InlineButton{Text: "✓ Подтвердить", Data: "tfa_approve_" + job.ChallengeID}
	denyBtn := tele.InlineButton{Text: "✕ Отклонить", Data: "tfa_deny_" + job.ChallengeID}
	markup := &tele.ReplyMarkup{
		InlineKeyboard: [][]tele.InlineButton{{approveBtn, denyBtn}},
	}
	_, err := b.bot.Send(&tele.User{ID: job.TelegramID}, job.Message, &tele.SendOptions{
		ParseMode:   tele.ModeHTML,
		ReplyMarkup: markup,
	})
	return err
}

// ─── Handler registration ─────────────────────────────────────────────────────

func (b *Bot) registerHandlers() {
	b.bot.Use(func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			if c.Sender() == nil {
				return next(c)
			}
			key := fmt.Sprintf("rl:bot:%d", c.Sender().ID)
			count, err := redisrepo.Increment(context.Background(), b.rdb, key, time.Minute)
			if err == nil && count > 20 {
				return c.Send("⏳ Слишком много запросов — подождите минуту.")
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
	b.bot.Handle("/newticket", b.handleNewTicket)
	b.bot.Handle("/help", b.handleHelp)
	b.bot.Handle("/link", b.handleLink)
	b.bot.Handle("/unlink", b.handleUnlink)
	b.bot.Handle("/info", b.handleInfo)
	b.bot.Handle("/resetpassword", b.handleResetPassword)
	b.bot.Handle("/devices", b.handleDevices)
	b.bot.Handle("/buydevice", b.handleBuyDevice)
	b.bot.Handle("/traffic", b.handleTraffic)
	b.bot.Handle("/history", b.handleHistory)
	b.bot.Handle("/payments", b.handlePayments)
	b.bot.Handle("/toggle2fa", b.handleToggle2FA)

	b.bot.Handle(tele.OnCallback, b.handleGenericCallback)
}

// ─── Brand helpers ────────────────────────────────────────────────────────────

const brandLine = "━━━━━━━━━━━━━━━━━━━━━"

func mainMenuText(user *domain.User, tgID int64, username string) string {
	name := username
	if name == "" {
		name = fmt.Sprintf("User%d", tgID)
	}
	adminLine := ""
	if user.IsAdmin {
		adminLine = "\n🔧 _Администратор_"
	}
	return fmt.Sprintf(
		"*MelloVPN* 🐍\n"+brandLine+"\n\n"+
			"Привет, *%s* 👋\n\n"+
			"🧪 Баланс: *%d ЯД*\n"+
			"🆔 ID: `%d`%s\n\n"+
			brandLine+"\nВыбери действие ↓",
		name,
		user.YADBalance,
		tgID,
		adminLine,
	)
}

func (b *Bot) mainMenuMarkup(user *domain.User) *tele.ReplyMarkup {
	rm := &tele.ReplyMarkup{}

	btnBuy := rm.Data("🛒 Купить VPN", "menu_buy")
	btnSubs := rm.Data("📋 Мои подписки", "menu_mysubs")
	btnDevices := rm.Data("📱 Устройства", "menu_devices")
	btnTraffic := rm.Data("📊 Трафик", "menu_traffic")
	btnTrial := rm.Data("🆓 Пробный период", "menu_trial")
	btnPromo := rm.Data("🎟 Промокод", "menu_promo")
	btnRef := rm.Data("👥 Рефералы", "menu_referrals")
	btnBalance := rm.Data("💰 Кошелёк", "menu_balance")
	btnHelp := rm.Data("❓ Помощь", "menu_help")
	btnSupport := rm.Data("🎫 Поддержка", "menu_support")
	btnInfo := rm.Data("ℹ️ О сервисе", "menu_info")

	rows := []tele.Row{
		rm.Row(btnBuy, btnSubs),
		rm.Row(btnDevices, btnTraffic),
		rm.Row(btnTrial, btnPromo),
		rm.Row(btnRef, btnBalance),
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
	return rm.Data("← Меню", "menu_back")
}

func (b *Bot) sendMainMenu(c tele.Context) error {
	ctx := context.Background()
	user, err := b.getUser(ctx, c)
	if err != nil {
		return c.Send("Ошибка: " + err.Error())
	}
	tgID := c.Sender().ID
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

	user, err := b.repo.GetByTelegramID(ctx, tgID)
	if err != nil {
		return c.Send("Произошла ошибка — попробуйте позже.")
	}

	if user != nil {
		if user.IsBanned {
			return c.Send("Аккаунт заблокирован.")
		}
		return c.Send(
			mainMenuText(user, tgID, username),
			&tele.SendOptions{ParseMode: tele.ModeMarkdown},
			b.mainMenuMarkup(user),
		)
	}

	referralCode := ""
	parts := strings.Fields(c.Text())
	if len(parts) > 1 {
		referralCode = parts[1]
	}

	login := fmt.Sprintf("tg%d", tgID)
	randPass, err := botRandPassword()
	if err != nil {
		b.log.Error("bot: generate random password failed", zap.Error(err))
		return c.Send("Произошла ошибка — попробуйте позже.")
	}

	newUser, err := b.auth.Register(ctx, service.RegisterInput{
		Username:     login,
		Password:     randPass,
		ReferralCode: referralCode,
		IP:           "",
	})
	if err != nil {
		if existing, lookupErr := b.repo.GetByUsername(ctx, login); lookupErr == nil && existing != nil && existing.TelegramID == nil {
			tgIDVal := tgID
			_ = b.repo.SetTelegramID(ctx, existing.ID, &tgIDVal)
			return c.Send(
				mainMenuText(existing, tgID, username),
				&tele.SendOptions{ParseMode: tele.ModeMarkdown},
				b.mainMenuMarkup(existing),
			)
		}
		b.log.Warn("bot registration failed", zap.Error(err), zap.Int64("tg_id", tgID))
		return c.Send("Не удалось создать аккаунт — попробуйте позже.")
	}

	tgIDVal := tgID
	if err := b.repo.SetTelegramID(ctx, newUser.ID, &tgIDVal); err != nil {
		b.log.Warn("set telegram id after registration", zap.Error(err))
	}

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
	webURL := b.cfg.WebAppURL
	if webURL == "" {
		webURL = "https://vpn-platform.ru"
	}
	_ = c.Send(
		fmt.Sprintf(
			"🐍 *Добро пожаловать в MelloVPN!*\n"+brandLine+"\n\n"+
				"Привет, *%s*! 🎉 Аккаунт создан.\n\n"+
				"🆓  Попробуйте VPN бесплатно\n"+
				"💰  Тарифы от *40 ₽/неделю*\n\n"+
				"🌐  Личный кабинет: [%s](%s)\n\n"+
				"_Выберите действие в меню ↓_",
			name, webURL, webURL,
		),
		&tele.SendOptions{ParseMode: tele.ModeMarkdown},
	)
	return c.Send(
		mainMenuText(newUser, tgID, username),
		&tele.SendOptions{ParseMode: tele.ModeMarkdown},
		b.mainMenuMarkup(newUser),
	)
}

// ─── /balance ─────────────────────────────────────────────────────────────────

func (b *Bot) handleBalance(c tele.Context) error {
	ctx := context.Background()
	user, err := b.getUser(ctx, c)
	if err != nil {
		return c.Send("Ошибка: " + err.Error())
	}

	rm := &tele.ReplyMarkup{}
	btnBuy := rm.Data("🛒 Купить VPN", "menu_buy")
	btnHistory := rm.Data("📜 История ЯД", "menu_history")
	btnPayments := rm.Data("💳 Платежи", "menu_payments")
	rm.Inline(
		rm.Row(btnBuy),
		rm.Row(btnHistory, btnPayments),
		rm.Row(backBtn(rm)),
	)

	return c.Send(fmt.Sprintf(
		"*💰 Кошелёк*\n"+brandLine+"\n\n"+
			"🧪 Баланс: *%d ЯД*\n\n"+
			brandLine+"\n"+
			"*Как заработать ЯД:*\n"+
			"◽ Покупайте подписки — бонус ЯД\n"+
			"◽ Используйте промокоды",
		user.YADBalance,
	), &tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm)
}

// ─── /buy ─────────────────────────────────────────────────────────────────────

func (b *Bot) handleBuy(c tele.Context) error {
	ctx := context.Background()
	user, err := b.getUser(ctx, c)
	if err != nil {
		return c.Send("Ошибка: " + err.Error())
	}

	// Block renewal if active subscription has more than 15 days left.
	activeSub, _ := b.repo.GetActiveSubscription(ctx, user.ID)
	if activeSub != nil {
		daysLeft := int(time.Until(activeSub.ExpiresAt).Hours() / 24)
		if daysLeft > 15 {
			rm := &tele.ReplyMarkup{}
			rm.Inline(rm.Row(backBtn(rm)))
			return c.Send(fmt.Sprintf(
				"⏳ *Продление недоступно*\n"+brandLine+"\n\n"+
					"До окончания подписки ещё *%d дн.*\n\n"+
					"_Продление откроется за 15 дней до конца._",
				daysLeft,
			), &tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm)
		}
	}

	rm := &tele.ReplyMarkup{}
	btnWeek := rm.Data("1 нед · 40 ₽", "buy_1week")
	btnMonth := rm.Data("1 мес · 100 ₽", "buy_1month")
	btnThree := rm.Data("3 мес · 270 ₽", "buy_3months")
	btnWeekYAD := rm.Data(fmt.Sprintf("1 нед · %d ЯД", domain.PlanYADPrice(domain.PlanWeek)), "buyyad_1week")
	btnMonthYAD := rm.Data(fmt.Sprintf("1 мес · %d ЯД", domain.PlanYADPrice(domain.PlanMonth)), "buyyad_1month")
	btnThreeYAD := rm.Data(fmt.Sprintf("3 мес · %d ЯД", domain.PlanYADPrice(domain.PlanThreeMonth)), "buyyad_3months")
	rm.Inline(
		rm.Row(btnWeek, btnWeekYAD),
		rm.Row(btnMonth, btnMonthYAD),
		rm.Row(btnThree, btnThreeYAD),
		rm.Row(backBtn(rm)),
	)

	return c.Send(fmt.Sprintf(
		"🛒 *Купить VPN*\n"+brandLine+"\n\n"+
			"💳 *Рублями (СБП):*\n"+
			"  ▸ 1 неделя — *40 ₽* (+%d ЯД)\n"+
			"  ▸ 1 месяц — *100 ₽* (+%d ЯД)\n"+
			"  ▸ 3 месяца — *270 ₽* (+%d ЯД)\n\n"+
			"🧪 *За ЯД (моментально):*\n"+
			"  ▸ 1 неделя — *%d ЯД*\n"+
			"  ▸ 1 месяц — *%d ЯД*\n"+
			"  ▸ 3 месяца — *%d ЯД*\n\n"+
			brandLine+"\n🧪 Баланс: *%d ЯД*  ·  _Без автопродления_",
		domain.PlanYADBonus(domain.PlanWeek),
		domain.PlanYADBonus(domain.PlanMonth),
		domain.PlanYADBonus(domain.PlanThreeMonth),
		domain.PlanYADPrice(domain.PlanWeek),
		domain.PlanYADPrice(domain.PlanMonth),
		domain.PlanYADPrice(domain.PlanThreeMonth),
		user.YADBalance,
	), &tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm)
}

func (b *Bot) handleBuyRubles(plan domain.SubscriptionPlan) tele.HandlerFunc {
	return func(c tele.Context) error {
		ctx := context.Background()
		_ = c.Respond()
		user, err := b.getUser(ctx, c)
		if err != nil {
			return c.Send("Ошибка: " + err.Error())
		}

		activeSub, _ := b.repo.GetActiveSubscription(ctx, user.ID)
		if activeSub != nil {
			if daysLeft := int(time.Until(activeSub.ExpiresAt).Hours() / 24); daysLeft > 15 {
				rm := &tele.ReplyMarkup{}
				rm.Inline(rm.Row(backBtn(rm)))
				return c.Send(fmt.Sprintf(
					"⏳ *Продление недоступно*\n"+brandLine+"\n\n"+
						"До окончания подписки ещё *%d дн.*\n\n"+
						"_Продление откроется за 15 дней до конца._",
					daysLeft,
				), &tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm)
			}
		}

		redirect, payment, err := b.subSvc.InitiatePayment(ctx, user.ID, plan, b.cfg.PaymentReturnURL)
		if err != nil {
			return c.Send("Ошибка: " + err.Error())
		}

		rm := &tele.ReplyMarkup{}
		btnPay := rm.URL("💳 Перейти к оплате", redirect)
		rm.Inline(rm.Row(btnPay), rm.Row(backBtn(rm)))

		return c.Send(fmt.Sprintf(
			"💳 *Оплата подписки*\n"+brandLine+"\n\n"+
				"📦  Тариф: *%s*\n"+
				"💰  Сумма: *%.0f ₽*\n"+
				"🔖  Платёж: `%s`\n\n"+
				"_Нажмите кнопку для перехода к оплате._\n"+
				"_⏱ Ссылка действительна 15 минут._",
			planName(plan),
			float64(payment.AmountKopecks)/100,
			payment.ID.String(),
		), &tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm)
	}
}

func (b *Bot) handleBuyYAD(plan domain.SubscriptionPlan) tele.HandlerFunc {
	return func(c tele.Context) error {
		ctx := context.Background()
		_ = c.Respond()
		user, err := b.getUser(ctx, c)
		if err != nil {
			return c.Send("Ошибка: " + err.Error())
		}

		activeSub, _ := b.repo.GetActiveSubscription(ctx, user.ID)
		if activeSub != nil {
			if daysLeft := int(time.Until(activeSub.ExpiresAt).Hours() / 24); daysLeft > 15 {
				rm := &tele.ReplyMarkup{}
				rm.Inline(rm.Row(backBtn(rm)))
				return c.Send(fmt.Sprintf(
					"⏳ *Продление недоступно*\n"+brandLine+"\n\n"+
						"До окончания подписки ещё *%d дн.*\n\n"+
						"_Продление откроется за 15 дней до конца._",
					daysLeft,
				), &tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm)
			}
		}

		sub, err := b.ecoSvc.BuySubscriptionWithYAD(ctx, user.ID, plan)
		if err != nil {
			rm := &tele.ReplyMarkup{}
			rm.Inline(rm.Row(backBtn(rm)))
			return c.Send("Ошибка: "+err.Error(), rm)
		}

		rm := &tele.ReplyMarkup{}
		btnVPN := rm.Data("🔗 Подключить VPN", "get_vpn_link")
		rm.Inline(rm.Row(btnVPN), rm.Row(backBtn(rm)))

		return c.Send(fmt.Sprintf(
			"🎉 *Подписка активирована!*\n"+brandLine+"\n\n"+
				"📦  Тариф: *%s*\n"+
				"📅  До: `%s`\n"+
				"🧪  Оплачено: *%d ЯД*\n\n"+
				"_Подключите VPN через кнопку ниже_ 👇",
			planName(sub.Plan),
			sub.ExpiresAt.Format("02.01.2006 15:04"),
			domain.PlanYADPrice(plan),
		), &tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm)
	}
}

// ─── /mysubs ──────────────────────────────────────────────────────────────────

func (b *Bot) handleMySubs(c tele.Context) error {
	ctx := context.Background()
	user, err := b.getUser(ctx, c)
	if err != nil {
		return c.Send("Ошибка: " + err.Error())
	}

	subs, err := b.subSvc.GetUserSubscriptions(ctx, user.ID)

	rm := &tele.ReplyMarkup{}
	if err != nil || len(subs) == 0 {
		btnBuy := rm.Data("🛒 Купить VPN", "menu_buy")
		btnTrial := rm.Data("🆓 Пробный период", "menu_trial")
		rm.Inline(rm.Row(btnBuy, btnTrial), rm.Row(backBtn(rm)))
		return c.Send(
			"📋 *Мои подписки*\n"+brandLine+"\n\n"+
				"❌  Активных подписок нет.\n\n"+
				"_Попробуйте пробный период или купите тариф._",
			&tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm,
		)
	}

	msg := "📋 *Мои подписки*\n" + brandLine + "\n"
	for i, sub := range subs {
		status := "🟢 активна"
		switch sub.Status {
		case domain.SubStatusExpired:
			status = "🔴 истекла"
		case domain.SubStatusTrial:
			status = "🟡 пробная"
		case domain.SubStatusCanceled:
			status = "⚫ отменена"
		}
		daysLeft := int(time.Until(sub.ExpiresAt).Hours() / 24)
		daysStr := ""
		if sub.Status == domain.SubStatusActive || sub.Status == domain.SubStatusTrial {
			if daysLeft > 0 {
				daysStr = fmt.Sprintf(" · ⦇ %d дн.", daysLeft)
			} else {
				daysStr = " · _истекает сегодня_"
			}
		}
		msg += fmt.Sprintf(
			"\n  *%d.* *%s* — %s%s\n      📅 до `%s`\n",
			i+1, planName(sub.Plan), status, daysStr,
			sub.ExpiresAt.Format("02.01.2006"),
		)
	}

	btnVPN := rm.Data("🔗 Подключить VPN", "get_vpn_link")
	var rows []tele.Row
	rows = append(rows, rm.Row(btnVPN))
	// Show renew button only when ≤15 days remain on the active subscription.
	activeSub, _ := b.repo.GetActiveSubscription(ctx, user.ID)
	if activeSub == nil || int(time.Until(activeSub.ExpiresAt).Hours()/24) <= 15 {
		btnRenew := rm.Data("🔄 Продлить", "menu_buy")
		rows = append(rows, rm.Row(btnRenew))
	}
	rows = append(rows, rm.Row(backBtn(rm)))
	rm.Inline(rows...)
	return c.Send(msg, &tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm)
}

// ─── /promo ───────────────────────────────────────────────────────────────────

func (b *Bot) handlePromo(c tele.Context) error {
	ctx := context.Background()
	user, err := b.getUser(ctx, c)
	if err != nil {
		return c.Send("Ошибка: " + err.Error())
	}

	parts := strings.Fields(c.Text())
	if len(parts) < 2 {
		rm := &tele.ReplyMarkup{}
		rm.Inline(rm.Row(backBtn(rm)))
		return c.Send(
			"🎟 *Промокод*\n"+brandLine+"\n\n"+
				"✉️  Формат: `/promo КОД`\n"+
				"📝  Пример: `/promo SUMMER2024`",
			&tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm,
		)
	}
	code := strings.ToUpper(parts[1])

	promo, err := b.ecoSvc.UsePromoCode(ctx, user.ID, code)
	if err != nil {
		rm := &tele.ReplyMarkup{}
		rm.Inline(rm.Row(backBtn(rm)))
		return c.Send("Ошибка: "+err.Error(), rm)
	}

	rm := &tele.ReplyMarkup{}
	rm.Inline(rm.Row(backBtn(rm)))

	if promo.PromoType == domain.PromoTypeDiscount {
		return c.Send(fmt.Sprintf(
			"*✅ Промокод активирован*\n"+brandLine+"\n\n"+
				"Скидка *%d%%* на следующую покупку.",
			promo.DiscountPercent,
		), &tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm)
	}

	return c.Send(fmt.Sprintf(
		"*✅ Промокод активирован*\n"+brandLine+"\n\n"+
			"🧪 Начислено: *%d ЯД*",
		promo.YADAmount,
	), &tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm)
}

// ─── /referral ────────────────────────────────────────────────────────────────

func (b *Bot) handleReferral(c tele.Context) error {
	ctx := context.Background()
	user, err := b.getUser(ctx, c)
	if err != nil {
		return c.Send("Ошибка: " + err.Error())
	}

	refs, _ := b.repo.GetReferralsByReferrer(ctx, user.ID)
	referralLink := fmt.Sprintf("https://t.me/%s?start=%s", b.bot.Me.Username, user.ReferralCode)
	shareURL := fmt.Sprintf(
		"https://t.me/share/url?url=%s&text=MelloVPN%%20—%%20быстрый%%20и%%20безопасный%%20VPN",
		referralLink,
	)

	rm := &tele.ReplyMarkup{}
	btnShare := rm.URL("📤 Поделиться ссылкой", shareURL)
	rm.Inline(rm.Row(btnShare), rm.Row(backBtn(rm)))

	return c.Send(fmt.Sprintf(
		"👥 *Рефералы*\n"+brandLine+"\n\n"+
			"🔗  Ваша ссылка:\n`%s`\n\n"+
			"👤  Приглашено: *%d* чел.",
		referralLink, len(refs),
	), &tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm)
}

// ─── /trial ───────────────────────────────────────────────────────────────────

func (b *Bot) handleTrial(c tele.Context) error {
	webURL := b.cfg.WebAppURL
	if webURL == "" {
		webURL = "https://vpn-platform.ru"
	}

	rm := &tele.ReplyMarkup{}
	btnSite := rm.URL("🌐 Открыть сайт", webURL+"/register")
	rm.Inline(rm.Row(btnSite), rm.Row(backBtn(rm)))

	return c.Send(
		"🆓 *Пробный период*\n"+brandLine+"\n\n"+
			"Попробуйте VPN бесплатно:\n\n"+
			"  1️⃣  Зарегистрируйтесь на сайте\n"+
			"  2️⃣  Перейдите в «Подписки»\n"+
			"  3️⃣  Нажмите «Активировать»\n\n"+
			"🔓 _Без оплаты · Без привязки карты_",
		&tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm,
	)
}

// ─── /ticket (list) ──────────────────────────────────────────────────────────

func (b *Bot) handleTicketMenu(c tele.Context) error {
	ctx := context.Background()
	user, err := b.getUser(ctx, c)
	if err != nil {
		return c.Send("Ошибка: " + err.Error())
	}

	allTickets, _ := b.repo.ListTickets(ctx, &user.ID, "", 10, 0)
	// Show only open and answered tickets.
	var tickets []*domain.Ticket
	for _, t := range allTickets {
		if t.Status != domain.TicketClosed {
			tickets = append(tickets, t)
		}
	}
	rm := &tele.ReplyMarkup{}
	btnNew := rm.Data("✏️ Создать тикет", "menu_newticket")
	rm.Inline(rm.Row(btnNew), rm.Row(backBtn(rm)))

	if len(tickets) == 0 {
		return c.Send(
			"*🎫 Поддержка*\n"+brandLine+"\n\n"+
				"Открытых обращений нет.\n\n"+
				"Создайте тикет: `/newticket тема сообщения`",
			&tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm,
		)
	}

	msg := "*🎫 Поддержка*\n" + brandLine + "\n\n"
	for _, t := range tickets {
		status := "●"
		if t.Status == domain.TicketAnswered {
			status = "◉"
		}
		msg += fmt.Sprintf("%s `%s` — %s\n", status, t.ID.String()[:8], t.Subject)
	}
	msg += "\n_● открыт · ◉ ответ_"
	return c.Send(msg, &tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm)
}

// ─── /newticket ───────────────────────────────────────────────────────────────

func (b *Bot) handleNewTicket(c tele.Context) error {
	ctx := context.Background()
	user, err := b.getUser(ctx, c)
	if err != nil {
		return c.Send("Ошибка: " + err.Error())
	}

	// Limit: 1 open/answered ticket per user.
	openCount, err := b.repo.CountOpenTickets(ctx, user.ID)
	if err != nil {
		b.log.Error("handleNewTicket: count open tickets", zap.Error(err))
		return c.Send("Не удалось проверить тикеты — попробуйте позже.")
	}
	if openCount > 0 {
		rm := &tele.ReplyMarkup{}
		rm.Inline(rm.Row(backBtn(rm)))
		return c.Send(
			"⚠️ *У вас уже есть открытый тикет*\n"+brandLine+"\n\n"+
				"Дождитесь его закрытия или проверьте статус: /ticket",
			&tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm,
		)
	}

	parts := strings.SplitN(strings.TrimSpace(c.Text()), " ", 2)
	if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
		rm := &tele.ReplyMarkup{}
		rm.Inline(rm.Row(backBtn(rm)))
		return c.Send(
			"✏️ *Создать тикет*\n"+brandLine+"\n\n"+
				"✉️  Формат: `/newticket Тема сообщения`\n\n"+
				"📝  Пример:\n`/newticket Не подключается VPN на iPhone`",
			&tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm,
		)
	}

	text := strings.TrimSpace(parts[1])
	subject := text
	body := text
	if idx := strings.Index(text, "\n"); idx > 0 {
		subject = strings.TrimSpace(text[:idx])
		body = strings.TrimSpace(text[idx+1:])
	}
	if len(subject) > 100 {
		subject = subject[:100]
	}

	now := time.Now()
	ticket := &domain.Ticket{
		ID:        uuid.New(),
		UserID:    user.ID,
		Subject:   subject,
		Status:    domain.TicketOpen,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := b.repo.CreateTicket(ctx, ticket); err != nil {
		b.log.Error("handleNewTicket: create ticket", zap.Error(err))
		return c.Send("Не удалось создать тикет — попробуйте позже.")
	}

	msg := &domain.TicketMessage{
		ID:        uuid.New(),
		TicketID:  ticket.ID,
		SenderID:  user.ID,
		IsAdmin:   false,
		Body:      body,
		CreatedAt: now,
	}
	if err := b.repo.AddTicketMessage(ctx, msg); err != nil {
		b.log.Error("handleNewTicket: add message", zap.Error(err))
	}

	rm := &tele.ReplyMarkup{}
	rm.Inline(rm.Row(backBtn(rm)))
	return c.Send(fmt.Sprintf(
		"✅ *Тикет создан!*\n"+brandLine+"\n\n"+
			"📌  Тема: *%s*\n"+
			"🔖  ID: `%s`\n\n"+
			"_Ответ придёт в тикет — проверяйте через_ /ticket",
		subject, ticket.ID.String()[:8],
	), &tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm)
}

// ─── /devices ─────────────────────────────────────────────────────────────────

func (b *Bot) handleDevices(c tele.Context) error {
	ctx := context.Background()
	user, err := b.getUser(ctx, c)
	if err != nil {
		return c.Send("Ошибка: " + err.Error())
	}

	devices, err := b.devSvc.ListDevices(ctx, user.ID)
	if err != nil {
		b.log.Warn("handleDevices: list", zap.Error(err))
		return c.Send("Не удалось загрузить устройства — попробуйте позже.")
	}

	limit, err := b.repo.GetEffectiveDeviceLimit(ctx, user.ID)
	if err != nil {
		limit = domain.DeviceMaxPerUser
	}
	expansion, _ := b.repo.GetActiveDeviceExpansion(ctx, user.ID)

	sort.Slice(devices, func(i, j int) bool {
		return devices[i].CreatedAt.Before(devices[j].CreatedAt)
	})

	rm := &tele.ReplyMarkup{}

	if len(devices) == 0 {
		var rows []tele.Row
		if expansion == nil {
			btnExpand := rm.Data("🔓 Расширить устройства", "menu_buydevice")
			rows = append(rows, rm.Row(btnExpand))
		}
		rows = append(rows, rm.Row(backBtn(rm)))
		rm.Inline(rows...)
		return c.Send(fmt.Sprintf(
			"📱 *Устройства*\n"+brandLine+"\n\n"+
				"❌  Нет подключённых устройств.\n"+
				"🔒  Лимит: *%d*", limit),
			&tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm)
	}

	activeCount := 0
	msg := "📱 *Устройства*\n" + brandLine + "\n\n"
	for idx, d := range devices {
		blocked := idx >= limit
		status := "🟢"
		suffix := ""
		if blocked {
			status = "🔒"
			suffix = " — _заблокировано_"
		} else if d.IsActive {
			activeCount++
		} else {
			status = "⚪"
		}
		msg += fmt.Sprintf("  %s `%s`%s\n", status, d.DeviceName, suffix)
	}
	msg += fmt.Sprintf("\n📊  %d / %d активных", activeCount, limit)

	if expansion != nil {
		msg += fmt.Sprintf("\n✅  +%d устройств (до %s)", expansion.ExtraDevices, expansion.ExpiresAt.Format("02.01.2006"))
	}

	var rows []tele.Row
	if len(devices) >= limit {
		btnDisconnect := rm.Data("✕ Отключить устройство", "menu_disconnect_list")
		rows = append(rows, rm.Row(btnDisconnect))
	}
	if expansion == nil || expansion.ExtraDevices < domain.DeviceExpansionMaxExtra {
		btnExpand := rm.Data("🔓 Расширить устройства", "menu_buydevice")
		rows = append(rows, rm.Row(btnExpand))
	}
	rows = append(rows, rm.Row(backBtn(rm)))
	rm.Inline(rows...)
	return c.Send(msg, &tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm)
}

// ─── /buydevice ───────────────────────────────────────────────────────────────

func (b *Bot) handleBuyDevice(c tele.Context) error {
	ctx := context.Background()
	user, err := b.getUser(ctx, c)
	if err != nil {
		return c.Send("Ошибка: " + err.Error())
	}

	expansion, _ := b.repo.GetActiveDeviceExpansion(ctx, user.ID)

	msg := "🔓 *Расширение устройств*\n" + brandLine + "\n\n"
	if expansion != nil {
		msg += fmt.Sprintf("✅ Активно: +%d устройств (до %s)\n\n", expansion.ExtraDevices, expansion.ExpiresAt.Format("02.01.2006"))
	}
	msg += "+1 устройство:  *50 ЯД*  |  *150 ₽*\n"
	msg += "+2 устройства:  *90 ЯД*  |  *270 ₽*\n\n"
	if expansion != nil && expansion.ExtraDevices == 1 {
		msg += "_Апгрейд +1→+2: 40 ЯД / 120 ₽_\n\n"
	}
	msg += "_Расширение действует до конца текущей подписки._"

	rm := &tele.ReplyMarkup{}
	var rows []tele.Row

	if expansion == nil || expansion.ExtraDevices < 1 {
		btn1YAD := rm.Data("+1 за ЯД (50)", "buydev_yad_1")
		btn1Money := rm.Data("+1 за рубли (150₽)", "buydev_money_1")
		rows = append(rows, rm.Row(btn1YAD, btn1Money))
	}
	if expansion == nil || expansion.ExtraDevices < 2 {
		btn2YAD := rm.Data("+2 за ЯД (90)", "buydev_yad_2")
		btn2Money := rm.Data("+2 за рубли (270₽)", "buydev_money_2")
		rows = append(rows, rm.Row(btn2YAD, btn2Money))
	}
	rows = append(rows, rm.Row(backBtn(rm)))
	rm.Inline(rows...)

	return c.Send(msg, &tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm)
}

// ─── /traffic ─────────────────────────────────────────────────────────────────

func (b *Bot) handleTraffic(c tele.Context) error {
	ctx := context.Background()
	user, err := b.getUser(ctx, c)
	if err != nil {
		return c.Send("Ошибка: " + err.Error())
	}

	remnaUUID := ""
	if user.RemnaUserUUID != nil {
		remnaUUID = *user.RemnaUserUUID
	}
	if remnaUUID == "" {
		rm := &tele.ReplyMarkup{}
		rm.Inline(rm.Row(backBtn(rm)))
		return c.Send(
			"📊 *Трафик*\n"+brandLine+"\n\n"+
				"❌  Нет данных — VPN не настроен.\n\n"+
				"_Подключите VPN через «Мои подписки»._",
			&tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm,
		)
	}

	remnaUser, err := b.remna.GetUser(ctx, remnaUUID)
	if err != nil {
		b.log.Warn("handleTraffic: remnawave get user", zap.Error(err))
		rm := &tele.ReplyMarkup{}
		rm.Inline(rm.Row(backBtn(rm)))
		return c.Send("Не удалось загрузить данные — попробуйте позже.", rm)
	}

	used := remnaUser.UserTraffic.UsedTrafficBytes
	lifetime := remnaUser.UserTraffic.LifetimeUsedTrafficBytes
	limitBytes := remnaUser.TrafficLimitBytes

	rm := &tele.ReplyMarkup{}
	rm.Inline(rm.Row(backBtn(rm)))

	limitStr := "∞"
	percentStr := ""
	if limitBytes > 0 {
		limitStr = formatBytes(limitBytes)
		percent := float64(used) / float64(limitBytes) * 100
		percentStr = fmt.Sprintf(" (%.0f%%)", percent)
	}

	return c.Send(fmt.Sprintf(
		"📊 *Трафик*\n"+brandLine+"\n\n"+
			"⬆️  Использовано: *%s*%s\n"+
			"📎  Лимит: *%s*\n"+
			"🌐  За всё время: *%s*",
		formatBytes(used), percentStr,
		limitStr,
		formatBytes(lifetime),
	), &tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm)
}

// ─── /history ─────────────────────────────────────────────────────────────────

func (b *Bot) handleHistory(c tele.Context) error {
	ctx := context.Background()
	user, err := b.getUser(ctx, c)
	if err != nil {
		return c.Send("Ошибка: " + err.Error())
	}

	txs, err := b.repo.GetYADTransactions(ctx, user.ID, 15)
	if err != nil || len(txs) == 0 {
		rm := &tele.ReplyMarkup{}
		rm.Inline(rm.Row(backBtn(rm)))
		return c.Send(
			"📜 *История ЯД*\n"+brandLine+"\n\n❌  Транзакций нет.",
			&tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm,
		)
	}

	msg := "📜 *История ЯД*\n" + brandLine + "\n\n"
	for _, tx := range txs {
		sign := "+"
		if tx.Delta < 0 {
			sign = ""
		}
		txType := yadTxTypeName(tx.TxType)
		msg += fmt.Sprintf(
			"  ▸ %s*%d* ЯД — %s\n      `%s`\n",
			sign, tx.Delta, txType,
			tx.CreatedAt.Format("02.01 15:04"),
		)
	}
	msg += fmt.Sprintf("\n💎 Баланс: *%d ЯД*", user.YADBalance)

	rm := &tele.ReplyMarkup{}
	rm.Inline(rm.Row(backBtn(rm)))
	return c.Send(msg, &tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm)
}

// ─── /payments ────────────────────────────────────────────────────────────────

func (b *Bot) handlePayments(c tele.Context) error {
	ctx := context.Background()
	user, err := b.getUser(ctx, c)
	if err != nil {
		return c.Send("Ошибка: " + err.Error())
	}

	payments, err := b.subSvc.GetPendingPayments(ctx, user.ID)
	if err != nil || len(payments) == 0 {
		rm := &tele.ReplyMarkup{}
		rm.Inline(rm.Row(backBtn(rm)))
		return c.Send(
			"💳 *Платежи*\n"+brandLine+"\n\n✅  Незавершённых платежей нет.",
			&tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm,
		)
	}

	msg := "💳 *Незавершённые платежи*\n" + brandLine + "\n\n"
	rm := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, p := range payments {
		msg += fmt.Sprintf(
			"  ▸ *%s* — %.0f ₽\n      `%s` · до %s\n",
			planName(p.Plan),
			float64(p.AmountKopecks)/100,
			p.ID.String()[:8],
			p.ExpiresAt.Format("15:04"),
		)
		if p.RedirectURL != "" {
			btn := rm.URL("💳 Оплатить "+p.ID.String()[:8], p.RedirectURL)
			rows = append(rows, rm.Row(btn))
		}
	}
	rows = append(rows, rm.Row(backBtn(rm)))
	rm.Inline(rows...)
	return c.Send(msg, &tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm)
}

// ─── /toggle2fa ───────────────────────────────────────────────────────────────

func (b *Bot) handleToggle2FA(c tele.Context) error {
	ctx := context.Background()
	user, err := b.getUser(ctx, c)
	if err != nil {
		return c.Send("Ошибка: " + err.Error())
	}

	newState := !user.TFAEnabled
	if err := b.repo.SetTFAEnabled(ctx, user.ID, newState); err != nil {
		b.log.Error("handleToggle2FA", zap.Error(err))
		return c.Send("Не удалось изменить настройку — попробуйте позже.")
	}

	rm := &tele.ReplyMarkup{}
	rm.Inline(rm.Row(backBtn(rm)))

	status := "включена ✅"
	if !newState {
		status = "выключена ❌"
	}
	return c.Send(fmt.Sprintf(
		"🔐 *Двухфакторная аутентификация*\n"+brandLine+"\n\n"+
			"🛡  2FA: *%s*\n\n"+
			"_При входе на сайт вам придёт запрос_\n"+
			"_на подтверждение в этот бот._",
		status,
	), &tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm)
}

// ─── /help ────────────────────────────────────────────────────────────────────

func (b *Bot) handleHelp(c tele.Context) error {
	rm := &tele.ReplyMarkup{}
	rm.Inline(rm.Row(backBtn(rm)))
	return c.Send(
		"❓ *Помощь*\n"+brandLine+"\n\n"+
			"📦 *Подписка и VPN:*\n"+
			"  ▸ /buy — купить подписку (₽ или ЯД)\n"+
			"  ▸ /mysubs — список подписок\n"+
			"  ▸ /trial — пробный период\n\n"+
			"📱 *Устройства:*\n"+
			"  ▸ /devices — список устройств\n"+
			"  ▸ /traffic — статистика трафика\n\n"+
			"💎 *Кошелёк:*\n"+
			"  ▸ /balance — баланс ЯД\n"+
			"  ▸ /history — история транзакций\n"+
			"  ▸ /payments — незавершённые платежи\n"+
			"  ▸ /referral — реферальная программа\n"+
			"  ▸ /promo КОД — активировать промокод\n\n"+
			"🔑 *Аккаунт:*\n"+
			"  ▸ /resetpassword — сбросить пароль\n"+
			"  ▸ /toggle2fa — вкл/выкл 2FA\n"+
			"  ▸ /link КОД — привязать Telegram\n"+
			"  ▸ /unlink — отвязать Telegram\n\n"+
			"🎟 *Прочее:*\n"+
			"  ▸ /ticket — поддержка\n"+
			"  ▸ /newticket — создать тикет\n"+
			"  ▸ /info — документы и контакты",
		&tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm,
	)
}

// ─── /info ────────────────────────────────────────────────────────────────────

func (b *Bot) handleInfo(c tele.Context) error {
	webURL := b.cfg.WebAppURL
	if webURL == "" {
		webURL = "https://vpn-platform.ru"
	}

	rm := &tele.ReplyMarkup{}
	btnPrivacy := rm.URL("📄 Политика конфиденциальности", webURL+"/PrivacyPolicy")
	btnAgreement := rm.URL("📄 Пользовательское соглашение", webURL+"/UserAgreement")
	btnSupport := rm.URL("💬 Поддержка", "https://t.me/Mellow_support")
	rm.Inline(rm.Row(btnPrivacy), rm.Row(btnAgreement), rm.Row(btnSupport), rm.Row(backBtn(rm)))

	return c.Send(
		"ℹ️ *О сервисе MelloVPN*\n"+brandLine+"\n\n"+
			"🔐  Протокол: VLESS/Reality\n"+
			"📱  iOS, Android, Windows, macOS, Linux\n"+
			"🚫  Без рекламы и трекеров\n\n"+
			"💬  Поддержка: @Mellow\\_support",
		&tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm,
	)
}

// ─── /link ────────────────────────────────────────────────────────────────────

func (b *Bot) handleLink(c tele.Context) error {
	args := strings.Fields(c.Message().Text)
	if len(args) < 2 {
		return c.Send(
			"🔗 *Привязка аккаунта*\n"+brandLine+"\n\n"+
				"✉️  Формат: `/link КОД`\n\n"+
				"_Откройте сайт → Настройки → «Привязать Telegram»_\n"+
				"_и скопируйте команду._",
			&tele.SendOptions{ParseMode: tele.ModeMarkdown},
		)
	}
	code := strings.ToUpper(strings.TrimSpace(args[1]))
	key := fmt.Sprintf("tg:link:%s", code)

	ctx := context.Background()
	userIDStr, err := b.rdb.GetDel(ctx, key).Result()
	if err == redis.Nil {
		return c.Send("Код не найден или уже истёк. Запросите новый на сайте.")
	}
	if err != nil {
		b.log.Error("handleLink: redis getdel", zap.Error(err))
		return c.Send("Временная ошибка — попробуйте позже.")
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		b.log.Error("handleLink: invalid user id in redis", zap.String("value", userIDStr))
		return c.Send("Внутренняя ошибка — запросите новый код на сайте.")
	}

	tgID := c.Sender().ID

	existingUser, err := b.repo.GetByTelegramID(ctx, tgID)
	if err != nil {
		b.log.Error("handleLink: get by telegram id", zap.Error(err))
		return c.Send("Временная ошибка — попробуйте позже.")
	}

	if existingUser != nil && existingUser.ID == userID {
		return c.Send("Этот Telegram уже привязан к вашему аккаунту.")
	}

	if existingUser != nil && existingUser.ID != userID {
		// Merge: transfer all data from the old (bot-created) account to the new (website) account
		if err := b.repo.MergeUsers(ctx, existingUser.ID, userID); err != nil {
			b.log.Error("handleLink: merge users", zap.Error(err),
				zap.String("src", existingUser.ID.String()),
				zap.String("dst", userID.String()))
			return c.Send("Не удалось объединить аккаунты — попробуйте позже.")
		}
		b.log.Info("handleLink: merged accounts",
			zap.String("src", existingUser.ID.String()),
			zap.String("dst", userID.String()),
			zap.Int64("tg_id", tgID))
	}

	if err := b.repo.SetTelegramID(ctx, userID, &tgID); err != nil {
		b.log.Error("handleLink: set telegram id", zap.Error(err))
		return c.Send("Не удалось привязать аккаунт — попробуйте снова.")
	}

	_ = b.repo.CreateAccountActivity(ctx, &domain.AccountActivity{
		ID:        uuid.New(),
		UserID:    userID,
		EventType: "telegram_link",
		CreatedAt: time.Now(),
	})

	msg := "✅ *Telegram привязан!*\n" + brandLine + "\n\n" +
		"🔗  Теперь вы можете управлять\nподпиской прямо из бота."
	if existingUser != nil && existingUser.ID != userID {
		msg = "✅ *Аккаунты объединены!*\n" + brandLine + "\n\n" +
			"🔗  Telegram привязан к аккаунту сайта.\n" +
			"💎  Баланс ЯД, подписки и история перенесены."
	}

	return c.Send(msg, &tele.SendOptions{ParseMode: tele.ModeMarkdown})
}

// ─── /unlink ──────────────────────────────────────────────────────────────────

func (b *Bot) handleUnlink(c tele.Context) error {
	ctx := context.Background()
	tgID := c.Sender().ID

	rlKey := fmt.Sprintf("rl:unlink_bot:%d", tgID)
	count, rlErr := redisrepo.Increment(ctx, b.rdb, rlKey, 10*time.Minute)
	if rlErr == nil && count > 3 {
		return c.Send("⏳ Слишком много запросов — повторите через 10 минут.")
	}

	user, err := b.repo.GetByTelegramID(ctx, tgID)
	if err != nil {
		b.log.Error("handleUnlink: get user by tg id", zap.Error(err))
		return c.Send("Временная ошибка — попробуйте позже.")
	}
	if user == nil {
		return c.Send("К этому Telegram не привязан аккаунт.")
	}
	if user.IsBanned {
		return c.Send("Аккаунт заблокирован.")
	}

	code, err := botRandCode()
	if err != nil {
		b.log.Error("handleUnlink: generate code", zap.Error(err))
		return c.Send("Внутренняя ошибка — попробуйте позже.")
	}

	key := fmt.Sprintf("tg:unlink:%s", code)
	if err := b.rdb.Set(ctx, key, user.ID.String(), 10*time.Minute).Err(); err != nil {
		b.log.Error("handleUnlink: redis set", zap.Error(err))
		return c.Send("Временная ошибка — попробуйте позже.")
	}

	return c.Send(
		"🔓 *Отвязка Telegram*\n"+brandLine+"\n\n"+
			"🔑  Код: `"+code+"`\n\n"+
			"💻  Введите этот код на сайте:\n"+
			"*Настройки → Отвязать Telegram*\n\n"+
			"_⏱ Код действителен 10 минут._",
		&tele.SendOptions{ParseMode: tele.ModeMarkdown},
	)
}

// ─── /resetpassword ───────────────────────────────────────────────────────────

func (b *Bot) handleResetPassword(c tele.Context) error {
	ctx := context.Background()
	tgID := c.Sender().ID

	rlKey := fmt.Sprintf("rl:resetpw:%d", tgID)
	count, rlErr := redisrepo.Increment(ctx, b.rdb, rlKey, time.Hour)
	if rlErr == nil && count > 3 {
		return c.Send("⏳ Слишком много попыток — повторите через час.")
	}

	user, err := b.repo.GetByTelegramID(ctx, tgID)
	if err != nil {
		b.log.Error("handleResetPassword: get user by tg id", zap.Error(err))
		return c.Send("Временная ошибка — попробуйте позже.")
	}
	if user == nil {
		return c.Send("К этому Telegram не привязан аккаунт.\nПривяжите аккаунт через /link КОД")
	}
	if user.IsBanned {
		return c.Send("Аккаунт заблокирован.")
	}

	newPw, err := botRandPassword()
	if err != nil {
		b.log.Error("handleResetPassword: generate password", zap.Error(err))
		return c.Send("Внутренняя ошибка — попробуйте позже.")
	}

	hash, err := password.Hash(newPw)
	if err != nil {
		b.log.Error("handleResetPassword: hash password", zap.Error(err))
		return c.Send("Внутренняя ошибка — попробуйте позже.")
	}

	if err := b.repo.SetPassword(ctx, user.ID, hash); err != nil {
		b.log.Error("handleResetPassword: set password", zap.Error(err))
		return c.Send("Не удалось сбросить пароль — попробуйте позже.")
	}

	if vErr := redisrepo.SetPasswordVersion(ctx, b.rdb, user.ID.String(), time.Now()); vErr != nil {
		b.log.Warn("handleResetPassword: set password version", zap.Error(vErr))
	}

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
		"🔑 *Пароль сброшен!*\n"+brandLine+"\n\n"+
			"👤  Логин: `"+loginName+"`\n"+
			"🔒  Новый пароль: `"+newPw+"`\n\n"+
			"_Рекомендуем сменить пароль в настройках._\n"+
			"_Все активные сессии завершены._",
		&tele.SendOptions{ParseMode: tele.ModeMarkdown},
	)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func botRandPassword() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

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

func (b *Bot) getUser(ctx context.Context, c tele.Context) (*domain.User, error) {
	tgID := c.Sender().ID
	user, err := b.repo.GetByTelegramID(ctx, tgID)
	if err != nil {
		return nil, fmt.Errorf("ошибка базы данных")
	}
	if user == nil {
		return nil, fmt.Errorf("аккаунт не найден — используйте /start")
	}
	if user.IsBanned {
		return nil, fmt.Errorf("аккаунт заблокирован")
	}
	return user, nil
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

func yadTxTypeName(t domain.YADTxType) string {
	switch t {
	case domain.YADTxReferralReward:
		return "реферальный бонус"
	case domain.YADTxBonus:
		return "бонус"
	case domain.YADTxSpent:
		return "покупка"
	case domain.YADTxPromo:
		return "промокод"
	case domain.YADTxTrial:
		return "пробный период"
	default:
		return string(t)
	}
}

func formatBytes(byt int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
		tb = gb * 1024
	)
	switch {
	case byt >= tb:
		return fmt.Sprintf("%.1f ТБ", float64(byt)/float64(tb))
	case byt >= gb:
		return fmt.Sprintf("%.1f ГБ", float64(byt)/float64(gb))
	case byt >= mb:
		return fmt.Sprintf("%.1f МБ", float64(byt)/float64(mb))
	case byt >= kb:
		return fmt.Sprintf("%.0f КБ", float64(byt)/float64(kb))
	default:
		return fmt.Sprintf("%d Б", byt)
	}
}

func (b *Bot) SendNotification(tgID int64, message string) error {
	_, err := b.bot.Send(&tele.User{ID: tgID}, message)
	return err
}

// ─── RegisterBuyCallbacks ─────────────────────────────────────────────────────

func (b *Bot) RegisterBuyCallbacks() {
	// Buy plan callbacks (rubles)
	b.bot.Handle(&tele.Btn{Unique: "buy_1week"}, b.handleBuyRubles(domain.PlanWeek))
	b.bot.Handle(&tele.Btn{Unique: "buy_1month"}, b.handleBuyRubles(domain.PlanMonth))
	b.bot.Handle(&tele.Btn{Unique: "buy_3months"}, b.handleBuyRubles(domain.PlanThreeMonth))

	// Buy plan callbacks (YAD)
	b.bot.Handle(&tele.Btn{Unique: "buyyad_1week"}, b.handleBuyYAD(domain.PlanWeek))
	b.bot.Handle(&tele.Btn{Unique: "buyyad_1month"}, b.handleBuyYAD(domain.PlanMonth))
	b.bot.Handle(&tele.Btn{Unique: "buyyad_3months"}, b.handleBuyYAD(domain.PlanThreeMonth))

	// Disconnect device list
	b.bot.Handle(&tele.Btn{Unique: "menu_disconnect_list"}, func(c tele.Context) error {
		_ = c.Respond()
		ctx := context.Background()
		user, err := b.getUser(ctx, c)
		if err != nil {
			return c.Send("Ошибка: " + err.Error())
		}

		devices, err := b.devSvc.ListDevices(ctx, user.ID)
		if err != nil || len(devices) == 0 {
			rm := &tele.ReplyMarkup{}
			rm.Inline(rm.Row(backBtn(rm)))
			return c.Send("Нет устройств для отключения.", rm)
		}

		rm := &tele.ReplyMarkup{}
		var rows []tele.Row
		for _, d := range devices {
			name := d.DeviceName
			if len(name) > 25 {
				name = name[:25] + "…"
			}
			btn := rm.Data("✕ "+name, "disconnect_"+d.HwidID)
			rows = append(rows, rm.Row(btn))
		}
		rows = append(rows, rm.Row(backBtn(rm)))
		rm.Inline(rows...)

		return c.Send(
			"✕ *Отключить устройство*\n"+brandLine+"\n\n_Выберите устройство:_",
			&tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm,
		)
	})

	// ─── Main menu callbacks ─────────────────────────────────────────────

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
	b.bot.Handle(&tele.Btn{Unique: "menu_devices"}, func(c tele.Context) error {
		_ = c.Respond()
		return b.handleDevices(c)
	})
	b.bot.Handle(&tele.Btn{Unique: "menu_buydevice"}, func(c tele.Context) error {
		_ = c.Respond()
		return b.handleBuyDevice(c)
	})
	b.bot.Handle(&tele.Btn{Unique: "buydev_yad_1"}, func(c tele.Context) error {
		_ = c.Respond()
		return b.handleBuyDeviceYAD(c, 1)
	})
	b.bot.Handle(&tele.Btn{Unique: "buydev_yad_2"}, func(c tele.Context) error {
		_ = c.Respond()
		return b.handleBuyDeviceYAD(c, 2)
	})
	b.bot.Handle(&tele.Btn{Unique: "buydev_money_1"}, func(c tele.Context) error {
		_ = c.Respond()
		return b.handleBuyDeviceMoney(c, 1)
	})
	b.bot.Handle(&tele.Btn{Unique: "buydev_money_2"}, func(c tele.Context) error {
		_ = c.Respond()
		return b.handleBuyDeviceMoney(c, 2)
	})
	b.bot.Handle(&tele.Btn{Unique: "menu_traffic"}, func(c tele.Context) error {
		_ = c.Respond()
		return b.handleTraffic(c)
	})
	b.bot.Handle(&tele.Btn{Unique: "menu_promo"}, func(c tele.Context) error {
		_ = c.Respond()
		rm := &tele.ReplyMarkup{}
		rm.Inline(rm.Row(backBtn(rm)))
		return c.Send(
			"🎟 *Промокод*\n"+brandLine+"\n\n"+
				"✉️  Формат: `/promo КОД`\n"+
				"📝  Пример: `/promo SUMMER2024`",
			&tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm,
		)
	})
	b.bot.Handle(&tele.Btn{Unique: "menu_support"}, func(c tele.Context) error {
		_ = c.Respond()
		return b.handleTicketMenu(c)
	})
	b.bot.Handle(&tele.Btn{Unique: "menu_newticket"}, func(c tele.Context) error {
		_ = c.Respond()
		rm := &tele.ReplyMarkup{}
		rm.Inline(rm.Row(backBtn(rm)))
		return c.Send(
			"✏️ *Создать тикет*\n"+brandLine+"\n\n"+
				"✉️  Формат: `/newticket Тема сообщения`\n\n"+
				"📝  Пример:\n`/newticket Не подключается VPN на iPhone`",
			&tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm,
		)
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
	b.bot.Handle(&tele.Btn{Unique: "menu_history"}, func(c tele.Context) error {
		_ = c.Respond()
		return b.handleHistory(c)
	})
	b.bot.Handle(&tele.Btn{Unique: "menu_balance"}, func(c tele.Context) error {
		_ = c.Respond()
		return b.handleBalance(c)
	})
	b.bot.Handle(&tele.Btn{Unique: "menu_payments"}, func(c tele.Context) error {
		_ = c.Respond()
		return b.handlePayments(c)
	})
b.bot.Handle(&tele.Btn{Unique: "menu_yadshop"}, func(c tele.Context) error {
		_ = c.Respond()
		ctx := context.Background()
		user, err := b.getUser(ctx, c)
		if err != nil {
			return c.Send("Ошибка: " + err.Error())
		}
		webURL := b.cfg.WebAppURL
		if webURL == "" {
			webURL = "https://vpn-platform.ru"
		}
		rm := &tele.ReplyMarkup{}
		btnShop := rm.URL("🏪 Открыть магазин", webURL+"/shop")
		rm.Inline(rm.Row(btnShop), rm.Row(backBtn(rm)))
		return c.Send(fmt.Sprintf(
			"🏪 *Магазин ЯД*\n"+brandLine+"\n\n"+
				"💎  Баланс: *%d ЯД*\n\n"+
				"💸 *Подписки за ЯД дешевле:*\n"+
				"  ▸ 1 неделя — *%d ЯД*\n"+
				"  ▸ 1 месяц — *%d ЯД*\n"+
				"  ▸ 3 месяца — *%d ЯД*",
			user.YADBalance,
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
		btnPanel := rm.URL("⚙️ Открыть панель", webURL+"/admin")
		rm.Inline(rm.Row(btnPanel), rm.Row(backBtn(rm)))
		return c.Send(
			"⚙️ *Панель администратора*\n"+brandLine+"\n\n"+
				"💻  Откройте веб-панель для\nуправления платформой.",
			&tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm,
		)
	})
	b.bot.Handle(&tele.Btn{Unique: "menu_back"}, func(c tele.Context) error {
		_ = c.Respond()
		return b.sendMainMenu(c)
	})

	// ─── VPN link ────────────────────────────────────────────────────────

	b.bot.Handle(&tele.Btn{Unique: "get_vpn_link"}, func(c tele.Context) error {
		_ = c.Respond()
		ctx := context.Background()
		user, err := b.getUser(ctx, c)
		if err != nil {
			return c.Send("Ошибка: " + err.Error())
		}

		remnaUUID := ""
		if user.RemnaUserUUID != nil {
			remnaUUID = *user.RemnaUserUUID
		}

		if remnaUUID == "" {
			subs, subErr := b.repo.GetActiveSubscription(ctx, user.ID)
			if subErr != nil || subs == nil {
				return c.Send("Активная подписка не найдена.\nКупите подписку чтобы использовать VPN.")
			}

			if subs.RemnaSubUUID != nil && *subs.RemnaSubUUID != "" {
				remnaUUID = *subs.RemnaSubUUID
				_ = b.repo.UpdateRemnaUUID(ctx, user.ID, remnaUUID)
			} else {
				remnaName := user.RemnaUsername()
				remnaUser, lookupErr := b.remna.GetUserByUsername(ctx, remnaName)
				if lookupErr != nil || remnaUser == nil || remnaUser.UUID == "" {
					remnaUser, lookupErr = b.remna.GetUserByUsername(ctx, user.ID.String())
				}
				if lookupErr == nil && remnaUser != nil && remnaUser.UUID != "" {
					remnaUUID = remnaUser.UUID
					_ = b.repo.UpdateRemnaUUID(ctx, user.ID, remnaUUID)
					return b.sendVPNLink(c, remnaUser.SubscribeURL)
				}

				remnaUser, createErr := b.remna.CreateUser(ctx, remnaName, subs.ExpiresAt)
				if createErr != nil || remnaUser == nil || remnaUser.UUID == "" {
					b.log.Warn("remnawave lazy repair: create user failed", zap.Error(createErr))
					return c.Send("Не удалось настроить VPN-аккаунт — попробуйте позже.")
				}
				remnaUUID = remnaUser.UUID
				_ = b.repo.UpdateRemnaUUID(ctx, user.ID, remnaUUID)
				return b.sendVPNLink(c, remnaUser.SubscribeURL)
			}
		}

		remnaUser, err := b.remna.GetUser(ctx, remnaUUID)
		if err != nil {
			b.log.Warn("remnawave get user", zap.Error(err))
			return c.Send("Не удалось загрузить данные подключения — попробуйте позже.")
		}

		return b.sendVPNLink(c, remnaUser.SubscribeURL)
	})
}

func (b *Bot) sendVPNLink(c tele.Context, url string) error {
	rm := &tele.ReplyMarkup{}
	btnOpen := rm.URL("🔗 Открыть ссылку", url)
	rm.Inline(rm.Row(btnOpen), rm.Row(backBtn(rm)))
	return c.Send(
		"🌐 *Подключение VPN*\n"+brandLine+"\n\n"+
			"`"+url+"`\n\n"+
			"_Вставьте в Happ, V2RayN, Hiddify или другой клиент._",
		&tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm,
	)
}

// ─── Generic callback handler ────────────────────────────────────────────────

func (b *Bot) handleGenericCallback(c tele.Context) error {
	data := c.Callback().Data
	switch {
	case strings.HasPrefix(data, "tfa_approve_"):
		return b.handleTFACallback(c, data[len("tfa_approve_"):], true)
	case strings.HasPrefix(data, "tfa_deny_"):
		return b.handleTFACallback(c, data[len("tfa_deny_"):], false)
	case strings.HasPrefix(data, "disconnect_"):
		return b.handleDisconnectCallback(c, data[len("disconnect_"):])
	default:
		return nil
	}
}

func (b *Bot) handleTFACallback(c tele.Context, challengeID string, approve bool) error {
	ctx := context.Background()
	_ = c.Respond()

	userID, status, err := redisrepo.Get2FAChallenge(ctx, b.rdb, challengeID)
	if err != nil || userID == "" {
		return c.Send("Запрос на вход истёк или уже обработан.")
	}
	if status != redisrepo.TFAPending {
		return c.Send("Этот запрос уже обработан.")
	}

	uid, err := uuid.Parse(userID)
	if err != nil {
		return c.Send("Некорректный запрос.")
	}
	user, err := b.repo.GetByID(ctx, uid)
	if err != nil || user == nil || user.TelegramID == nil || *user.TelegramID != c.Sender().ID {
		return c.Send("Вы не можете подтвердить этот запрос.")
	}

	newStatus := redisrepo.TFADenied
	text := "❌ *Вход отклонён*\n\n_Попытка входа заблокирована._"
	if approve {
		newStatus = redisrepo.TFAApproved
		text = "✅ *Вход подтверждён*\n\n_Вы успешно авторизованы._"
	}

	if err := redisrepo.Resolve2FAChallenge(ctx, b.rdb, challengeID, newStatus); err != nil {
		return c.Send("Ошибка обработки — попробуйте ещё раз.")
	}

	return c.Send(text, &tele.SendOptions{ParseMode: tele.ModeMarkdown})
}

func (b *Bot) handleDisconnectCallback(c tele.Context, hwidID string) error {
	ctx := context.Background()
	_ = c.Respond()

	user, err := b.getUser(ctx, c)
	if err != nil {
		return c.Send("Ошибка: " + err.Error())
	}

	if err := b.devSvc.DisconnectDevice(ctx, user.ID, hwidID); err != nil {
		rm := &tele.ReplyMarkup{}
		rm.Inline(rm.Row(backBtn(rm)))
		return c.Send("Ошибка: "+err.Error(), rm)
	}

	rm := &tele.ReplyMarkup{}
	btnDevices := rm.Data("📱 Устройства", "menu_devices")
	rm.Inline(rm.Row(btnDevices), rm.Row(backBtn(rm)))
	return c.Send("✅ *Устройство отключено*", &tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm)
}

func (b *Bot) handleBuyDeviceYAD(c tele.Context, qty int) error {
	ctx := context.Background()
	user, err := b.getUser(ctx, c)
	if err != nil {
		return c.Send("Ошибка: " + err.Error())
	}

	expansion, err := b.ecoSvc.BuyDeviceExpansion(ctx, user.ID, qty)
	if err != nil {
		rm := &tele.ReplyMarkup{}
		rm.Inline(rm.Row(backBtn(rm)))
		return c.Send("❌ "+err.Error(), rm)
	}

	rm := &tele.ReplyMarkup{}
	btnDevices := rm.Data("📱 Устройства", "menu_devices")
	rm.Inline(rm.Row(btnDevices), rm.Row(backBtn(rm)))

	return c.Send(fmt.Sprintf(
		"✅ *Расширение активировано*\n"+brandLine+"\n\n"+
			"+%d устройств до *%s*",
		expansion.ExtraDevices, expansion.ExpiresAt.Format("02.01.2006")),
		&tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm)
}

func (b *Bot) handleBuyDeviceMoney(c tele.Context, qty int) error {
	ctx := context.Background()
	user, err := b.getUser(ctx, c)
	if err != nil {
		return c.Send("Ошибка: " + err.Error())
	}

	redirectURL, _, err := b.subSvc.InitiateDeviceExpansionPayment(ctx, user.ID, qty, b.cfg.PaymentReturnURL)
	if err != nil {
		rm := &tele.ReplyMarkup{}
		rm.Inline(rm.Row(backBtn(rm)))
		return c.Send("❌ "+err.Error(), rm)
	}

	rm := &tele.ReplyMarkup{}
	btnPay := rm.URL("💳 Оплатить", redirectURL)
	rm.Inline(rm.Row(btnPay), rm.Row(backBtn(rm)))

	return c.Send(fmt.Sprintf(
		"💳 *Оплата расширения устройств*\n"+brandLine+"\n\n"+
			"➕ %d устройств\n"+
			"💰 %d ₽\n\n"+
			"_Нажмите кнопку для оплаты._",
		qty, domain.DeviceExpansionKopecks(qty)/100),
		&tele.SendOptions{ParseMode: tele.ModeMarkdown}, rm)
}

// Unused import avoidance
var _ = platega.MethodSBPQR
