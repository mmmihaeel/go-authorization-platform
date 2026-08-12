package broker

import (
	"context"
	"log/slog"

	"github.com/mmmihaeel/go-authorization-platform/internal/audit"
)

type LogPublisher struct{ Logger *slog.Logger }

func (p LogPublisher) Publish(_ context.Context, message audit.OutboxMessage) error {
	logger := p.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Info("published audit event", "event_id", message.ID, "event_type", message.EventType, "routing_key", message.RoutingKey, "bytes", len(message.Payload))
	return nil
}
