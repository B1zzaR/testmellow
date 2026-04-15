package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"github.com/vpnplatform/internal/anticheat"
	"github.com/vpnplatform/internal/bot"
	"github.com/vpnplatform/internal/config"
	"github.com/vpnplatform/internal/integration/platega"
	"github.com/vpnplatform/internal/integration/remnawave"
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

	db, err := dbpkg.New(ctx, cfg.DB)
	if err != nil {
		log.Fatal("connect db", zap.Error(err))
	}
	defer db.Close()

	rdb := redisrepo.New(cfg.Redis)
	defer rdb.Close()

	userRepo := dbpkg.NewUserRepo(db)
	platClient := platega.NewClient(cfg.Platega, log)
	remnaClient := remnawave.NewClient(cfg.Remna, log)
	antiEngine := anticheat.NewEngine(rdb, log)

	authSvc := service.NewAuthService(userRepo, antiEngine, rdb, log, cfg.App.AdminLogin)
	subSvc := service.NewSubscriptionService(userRepo, platClient, remnaClient, antiEngine, rdb, log)
	ecoSvc := service.NewEconomyService(userRepo, remnaClient, antiEngine, log)
	trialSvc := service.NewTrialService(userRepo, remnaClient, log)

	jwtMgr := jwtpkg.NewManager(cfg.JWT.Secret, cfg.JWT.AccessTTLHours)

	botCfg := bot.BotConfig{
		Token:     cfg.Telegram.Token,
		AdminID:   cfg.Telegram.AdminID,
		WebAppURL: cfg.Telegram.WebAppURL,
	}
	b, err := bot.New(botCfg, userRepo, authSvc, subSvc, ecoSvc, trialSvc, remnaClient, jwtMgr, rdb, log)
	if err != nil {
		log.Fatal("init bot", zap.Error(err))
	}

	b.RegisterBuyCallbacks()

	log.Info("telegram bot starting")
	go b.StartQueues(ctx)
	go b.Start()

	<-ctx.Done()
	log.Info("telegram bot stopping")
	b.Stop()
}
