package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/vpnplatform/internal/anticheat"
	"github.com/vpnplatform/internal/config"
	adminHandler "github.com/vpnplatform/internal/handler/admin"
	httpHandler "github.com/vpnplatform/internal/handler/http"
	webhookHandler "github.com/vpnplatform/internal/handler/webhook"
	"github.com/vpnplatform/internal/integration/platega"
	"github.com/vpnplatform/internal/integration/remnawave"
	"github.com/vpnplatform/internal/middleware"
	dbpkg "github.com/vpnplatform/internal/repository/postgres"
	redisrepo "github.com/vpnplatform/internal/repository/redis"
	"github.com/vpnplatform/internal/service"
	jwtpkg "github.com/vpnplatform/pkg/jwt"
	"github.com/vpnplatform/pkg/logger"
)

func main() {
	cfg := config.Load()

	log, err := logger.New(cfg.App.Env)
	if err != nil {
		panic(err)
	}
	defer log.Sync()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// ── Database ──────────────────────────────────────────────────────────
	db, err := dbpkg.New(ctx, cfg.DB)
	if err != nil {
		log.Fatal("connect db", zap.Error(err))
	}
	defer db.Close()

	// ── Run pending migrations ────────────────────────────────────────────
	if err := dbpkg.RunMigrations(ctx, db); err != nil {
		log.Fatal("run migrations", zap.Error(err))
	}

	// ── Redis ─────────────────────────────────────────────────────────────
	rdb := redisrepo.New(cfg.Redis)
	defer rdb.Close()

	// ── External clients ──────────────────────────────────────────────────
	platClient := platega.NewClient(cfg.Platega, log)
	remnaClient := remnawave.NewClient(cfg.Remna)

	// ── Repositories & services ───────────────────────────────────────────
	userRepo := dbpkg.NewUserRepo(db)
	deviceRepo := dbpkg.NewDeviceRepo(db)
	antiEngine := anticheat.NewEngine(rdb, log)

	authSvc := service.NewAuthService(userRepo, antiEngine, log, cfg.App.AdminLogin)
	subSvc := service.NewSubscriptionService(userRepo, platClient, remnaClient, antiEngine, rdb, log)
	ecoSvc := service.NewEconomyService(userRepo, remnaClient, antiEngine, log)
	trialSvc := service.NewTrialService(userRepo, remnaClient, log)
	deviceSvc := service.NewDeviceService(deviceRepo, userRepo, log)

	// ── JWT ───────────────────────────────────────────────────────────────
	jwtMgr := jwtpkg.NewManager(cfg.JWT.Secret, cfg.JWT.AccessTTLHours)

	// ── Handlers ──────────────────────────────────────────────────────────
	authH := httpHandler.NewAuthHandler(authSvc, jwtMgr, rdb, log)
	profileH := httpHandler.NewProfileHandler(userRepo, remnaClient, rdb, cfg.Telegram.BotUsername, log)
	balanceH := httpHandler.NewBalanceHandler(userRepo, log)
	subH := httpHandler.NewSubscriptionHandler(subSvc, log)
	paymentH := httpHandler.NewPaymentHandler(subSvc, userRepo, log)
	referralH := httpHandler.NewReferralHandler(userRepo, log)
	promoH := httpHandler.NewPromoHandler(ecoSvc, userRepo, log)
	trialH := httpHandler.NewTrialHandler(trialSvc, log)
	deviceH := httpHandler.NewDeviceHandler(deviceSvc, log)
	ticketH := httpHandler.NewTicketHandler(userRepo, log)
	shopH := httpHandler.NewShopHandler(userRepo, ecoSvc, log)
	webhookH := webhookHandler.NewPlategalHandler(platClient, userRepo, rdb, log)
	adminH := adminHandler.NewHandler(userRepo, rdb, antiEngine, platClient, remnaClient, log)

	// ── Gin Router ────────────────────────────────────────────────────────
	if cfg.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(
		middleware.Recovery(log),
		middleware.RequestID(),
		middleware.Logger(log),
		middleware.SecurityHeaders(),
		middleware.CORS(strings.Split(cfg.App.AllowedOrigins, ",")),
		middleware.MaxBodySize(1<<20), // 1 MiB
	)

	// Suppress browser noise
	r.GET("/favicon.ico", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	// Health check (no auth)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "ts": time.Now().Unix()})
	})

	// Webhook (no JWT — authenticated by Platega headers)
	r.POST(cfg.App.WebhookPath, webhookH.Handle)

	// Public auth endpoints
	auth := r.Group("/api/auth")
	{
		auth.POST("/register", middleware.IPRateLimit(rdb, "register", 5, time.Hour), authH.Register)
		auth.POST("/login", middleware.IPRateLimit(rdb, "login", 10, 15*time.Minute), authH.Login)
		auth.POST("/refresh", authH.Refresh)
		auth.POST("/logout", authH.Logout)
	}

	// Protected user endpoints
	api := r.Group("/api", middleware.Auth(jwtMgr, rdb), middleware.BannedCheck(rdb), middleware.UserRateLimit(rdb, 120, time.Minute))
	{
		api.GET("/profile", profileH.Get)
		api.GET("/profile/connection", profileH.GetConnection)
		api.GET("/profile/traffic", profileH.GetTraffic)
		api.POST("/profile/password", profileH.ChangePassword)
		api.PUT("/profile/telegram", profileH.UpdateTelegram)
		api.POST("/profile/telegram/link-code", profileH.GenerateLinkCode)
		api.GET("/balance", balanceH.Get)
		api.GET("/balance/history", balanceH.History)

		api.GET("/subscriptions", subH.List)
		api.GET("/subscriptions/:id", subH.GetByID)
		api.POST("/subscriptions/buy", subH.Buy)
		api.POST("/subscriptions/renew", subH.Renew)

		api.GET("/payments/pending", paymentH.ListPending)
		api.GET("/payments/history", paymentH.ListHistory)
		api.GET("/payments/:id", paymentH.GetByID)
		api.POST("/payments/:id/check", paymentH.Check)

		api.GET("/referrals", referralH.List)

		api.POST("/promo/use", middleware.IPRateLimit(rdb, "promo", 10, time.Hour), promoH.Use)
		api.GET("/promo/discount/active", promoH.ActiveDiscount)
		api.POST("/trial/activate", trialH.Activate)

		api.GET("/tickets", ticketH.List)
		api.POST("/tickets", ticketH.Create)
		api.GET("/tickets/:id", ticketH.GetWithMessages)
		api.POST("/tickets/:id/reply", ticketH.Reply)

		api.GET("/shop", shopH.List)
		api.POST("/shop/buy", shopH.Buy)
		api.POST("/shop/buy-subscription", shopH.BuySubscription)

		api.GET("/devices", deviceH.List)
		api.POST("/devices/:id/disconnect", deviceH.Disconnect)
	}

	// Admin endpoints (JWT required + is_admin flag, DB-verified on each request — H-4)
	adm := r.Group("/api/admin",
		middleware.Auth(jwtMgr, rdb),
		middleware.AdminDBCheck(userRepo.IsAdmin),
		middleware.AdminRateLimit(rdb, 300, time.Minute),
	)
	{
		adm.GET("/users", adminH.ListUsers)
		adm.GET("/users/:id", adminH.GetUser)
		adm.POST("/users/:id/ban", adminH.BanUser)
		adm.POST("/users/:id/unban", adminH.UnbanUser)
		adm.POST("/users/:id/reset-payment-limit", adminH.ResetPaymentLimit)
		adm.PATCH("/users/:id/risk", adminH.SetRiskScore)

		adm.GET("/analytics", adminH.Analytics)

		adm.GET("/promocodes", adminH.ListPromoCodes)
		adm.POST("/promocodes", adminH.CreatePromoCode)

		adm.GET("/tickets", adminH.ListTickets)
		adm.GET("/tickets/:id", adminH.GetTicket)
		adm.POST("/tickets/:id/reply", adminH.ReplyToTicket)
		adm.POST("/tickets/:id/close", adminH.CloseTicket)

		adm.POST("/shop/items", adminH.CreateShopItem)

		// Payments
		adm.GET("/payments", adminH.ListPayments)
		adm.GET("/payments/:id", adminH.GetPayment)
		adm.POST("/payments/:id/check", adminH.CheckPaymentStatus)

		// Subscriptions
		adm.POST("/subscriptions/assign", adminH.AssignSubscription)
		adm.GET("/subscriptions", adminH.ListSubscriptions)
		adm.PATCH("/subscriptions/:id/status", adminH.SetSubscriptionStatus)
		adm.POST("/subscriptions/:id/extend", adminH.ExtendSubscription)

		// YAD economy
		adm.GET("/yad", adminH.ListYADTransactions)
		adm.POST("/yad/adjust", adminH.AdjustYAD)

		// Referrals
		adm.GET("/referrals", adminH.ListReferrals)

		// User sub-resources
		adm.GET("/users/:id/subscriptions", adminH.GetUserSubscriptions)
		adm.GET("/users/:id/payments", adminH.GetUserPayments)
		adm.GET("/users/:id/yad", adminH.GetUserYAD)
		adm.POST("/users/:id/adjust-yad", adminH.AdjustUserYAD)

		// Analytics
		adm.GET("/analytics/revenue", adminH.RevenueAnalytics)

		// Audit logs
		adm.GET("/audit-logs", adminH.ListAuditLogs)
	}

	// ── HTTP Server ───────────────────────────────────────────────────────
	srv := &http.Server{
		Addr:         ":" + cfg.App.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("API server starting", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	<-ctx.Done()
	log.Info("shutting down API server")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown error", zap.Error(err))
	}
}
