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
)

type demoOutbox struct{ emitted bool }

func (d *demoOutbox) ClaimPending(_ context.Context, _ int) ([]audit.OutboxMessage, error) {
	if d.emitted {
		return nil, nil
	}
	d.emitted = true
	return []audit.OutboxMessage{{ID: "demo-event", EventType: "authorization.decision.v1", RoutingKey: "authorization.decision", Payload: []byte(`{"demo":true}`), CreatedAt: time.Now().UTC()}}, nil
}
func (*demoOutbox) MarkPublished(context.Context, string) error                { return nil }
func (*demoOutbox) MarkFailed(context.Context, string, error, time.Time) error { return nil }

var version = "dev"
var commit = "unknown"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// The default command demonstrates worker lifecycle and concurrency without
	// infrastructure. Production wiring uses SQLRepository + RabbitMQPublisher.
	worker, err := audit.NewWorker(&demoOutbox{}, broker.LogPublisher{Logger: logger}, audit.WorkerConfig{PollInterval: 2 * time.Second, Concurrency: 4})
	if err != nil {
		logger.Error("create worker", "error", err)
		os.Exit(1)
	}
	logger.Info("audit worker started", "mode", "demo", "version", version, "commit", commit)
	if err := worker.Run(ctx); err != nil && err != context.Canceled {
		logger.Error("worker stopped", "error", err)
		os.Exit(1)
	}
}
