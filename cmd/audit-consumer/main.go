package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mmmihaeel/go-authorization-platform/internal/config"
	"github.com/mmmihaeel/go-authorization-platform/internal/events"
	"github.com/mmmihaeel/go-authorization-platform/internal/platform"
)

var version = "dev"
var commit = "unknown"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	cfg, err := config.Load()
	if err != nil {
		logger.Error("load configuration", "error", err)
		os.Exit(1)
	}
	db, err := platform.OpenPostgres(cfg.DatabaseURL)
	if err != nil {
		logger.Error("connect audit database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	inbox, err := events.NewSQLInbox(db, "audit-log", 2*time.Minute)
	if err != nil {
		logger.Error("configure inbox", "error", err)
		os.Exit(1)
	}
	handler, err := events.NewSQLAuditHandler(db)
	if err != nil {
		logger.Error("configure audit handler", "error", err)
		os.Exit(1)
	}
	consumer, err := events.NewConsumer(inbox, handler)
	if err != nil {
		logger.Error("configure consumer", "error", err)
		os.Exit(1)
	}
	logger.Info("audit consumer started", "version", version, "commit", commit)
	err = events.RunRabbitMQ(ctx, events.RabbitMQConfig{
		URL: cfg.RabbitMQURL, Exchange: cfg.RabbitMQExchange, Queue: cfg.RabbitMQQueue,
		Concurrency: 4,
	}, consumer, logger)
	if err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("audit consumer stopped", "error", err)
		os.Exit(1)
	}
}
