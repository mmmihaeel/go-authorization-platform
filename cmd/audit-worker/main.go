package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mmmihaeel/go-authorization-platform/internal/audit"
	"github.com/mmmihaeel/go-authorization-platform/internal/broker"
	"github.com/mmmihaeel/go-authorization-platform/internal/config"
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
	repository, err := audit.NewSQLRepository(db)
	if err != nil {
		logger.Error("configure outbox repository", "error", err)
		os.Exit(1)
	}
	publisher, err := broker.NewRabbitMQPublisher(cfg.RabbitMQURL, cfg.RabbitMQExchange)
	if err != nil {
		logger.Error("connect RabbitMQ publisher", "error", err)
		os.Exit(1)
	}
	defer publisher.Close()

	worker, err := audit.NewWorker(repository, publisher, audit.WorkerConfig{
		PollInterval: time.Second, BatchSize: 50, Concurrency: 4,
		OnError: func(err error) { logger.Error("dispatch outbox batch", "error", err) },
	})
	if err != nil {
		logger.Error("create worker", "error", err)
		os.Exit(1)
	}
	logger.Info("audit outbox worker started", "version", version, "commit", commit)
	if err := worker.Run(ctx); err != nil && err != context.Canceled {
		logger.Error("worker stopped", "error", err)
		os.Exit(1)
	}
}
