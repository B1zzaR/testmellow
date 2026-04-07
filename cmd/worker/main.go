package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"github.com/vpnplatform/internal/anticheat"
	"github.com/vpnplatform/internal/config"
	"github.com/vpnplatform/internal/integration/platega"
	"github.com/vpnplatform/internal/integration/remnawave"
	dbpkg "github.com/vpnplatform/internal/repository/postgres"
	redisrepo "github.com/vpnplatform/internal/repository/redis"
	"github.com/vpnplatform/internal/worker"
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
	remnaClient := remnawave.NewClient(cfg.Remna)
	platClient := platega.NewClient(cfg.Platega, log)
	antiEngine := anticheat.NewEngine(rdb, log)

	w := worker.NewWorker(rdb, userRepo, remnaClient, platClient, antiEngine, cfg.Telegram.Token, log)

	log.Info("worker starting")
	w.Run(ctx)
	log.Info("worker stopped")
}
