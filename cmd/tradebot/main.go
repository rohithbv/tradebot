package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/rohithbv/tradebot/internal/broker"
	"github.com/rohithbv/tradebot/internal/config"
	"github.com/rohithbv/tradebot/internal/engine"
	"github.com/rohithbv/tradebot/internal/market"
	"github.com/rohithbv/tradebot/internal/notification"
	"github.com/rohithbv/tradebot/internal/storage"
	"github.com/rohithbv/tradebot/internal/strategy"
	"github.com/rohithbv/tradebot/internal/web"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to configuration file")
	flag.Parse()

	// Set up structured logging with JSON handler.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)

	// Load configuration.
	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}
	slog.Info("config loaded", "watchlist_size", len(cfg.Trading.Watchlist), "addr", cfg.Web.Addr)

	// Create context that cancels on SIGINT/SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Create components in dependency order.
	mktClient := market.NewMarketClient(cfg.Alpaca)

	store, err := storage.NewSQLiteStore(cfg.Storage.DBPath)
	if err != nil {
		slog.Error("failed to open storage", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	brk := broker.NewPaperBroker(cfg.Trading.InitialCash, store, cfg.Trading)

	strat := strategy.NewRSIMACDStrategy(
		cfg.Strategy.RSIPeriod,
		cfg.Strategy.RSIOversold,
		cfg.Strategy.RSIOverbought,
		cfg.Strategy.MACDFastPeriod,
		cfg.Strategy.MACDSlowPeriod,
		cfg.Strategy.MACDSignalPeriod,
	)

	var notifier notification.Notifier
	if cfg.Telegram.Enabled && cfg.Telegram.BotToken != "" && cfg.Telegram.ChatID != 0 {
		notifier = notification.NewTelegramNotifier(cfg.Telegram.BotToken, cfg.Telegram.ChatID)
		slog.Info("telegram notifications enabled")
	}

	eng := engine.New(mktClient, brk, strat, store, cfg, notifier)

	webSrv := web.NewServer(cfg.Web, brk, store)
	webSrv.SetEngine(eng)

	// Start engine in a goroutine.
	engineDone := make(chan error, 1)
	go func() {
		engineDone <- eng.Run(ctx)
	}()

	// Start web server in a goroutine.
	webDone := make(chan error, 1)
	go func() {
		webDone <- webSrv.Start(ctx)
	}()

	slog.Info("tradebot started", "dashboard", "http://localhost"+cfg.Web.Addr)

	// Wait for shutdown signal, then wait for components to finish.
	<-ctx.Done()
	slog.Info("shutdown signal received, stopping components...")

	// Wait for engine and web server to finish.
	if err := <-engineDone; err != nil && err != context.Canceled {
		slog.Error("engine error", "error", err)
	}
	if err := <-webDone; err != nil {
		slog.Error("web server error", "error", err)
	}

	slog.Info("tradebot stopped")
}
