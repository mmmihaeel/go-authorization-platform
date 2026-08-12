package events

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQConfig struct {
	URL, Exchange, Queue, RoutingKey string
	Concurrency                      int
}

func RunRabbitMQ(ctx context.Context, config RabbitMQConfig, consumer *Consumer, logger *slog.Logger) error {
	if consumer == nil || config.URL == "" {
		return fmt.Errorf("RabbitMQ URL and consumer are required")
	}
	if config.Exchange == "" {
		config.Exchange = "authorization.audit"
	}
	if config.Queue == "" {
		config.Queue = "authorization.audit.log"
	}
	if config.RoutingKey == "" {
		config.RoutingKey = "authorization.#"
	}
	if config.Concurrency <= 0 {
		config.Concurrency = 4
	}
	if logger == nil {
		logger = slog.Default()
	}
	connection, err := amqp.Dial(config.URL)
	if err != nil {
		return fmt.Errorf("connect RabbitMQ consumer: %w", err)
	}
	defer connection.Close()
	channel, err := connection.Channel()
	if err != nil {
		return fmt.Errorf("open RabbitMQ consumer channel: %w", err)
	}
	defer channel.Close()
	if err := channel.ExchangeDeclare(config.Exchange, "topic", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare audit exchange: %w", err)
	}
	queue, err := channel.QueueDeclare(config.Queue, true, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("declare audit queue: %w", err)
	}
	if err := channel.QueueBind(queue.Name, config.RoutingKey, config.Exchange, false, nil); err != nil {
		return fmt.Errorf("bind audit queue: %w", err)
	}
	if err := channel.Qos(config.Concurrency, 0, false); err != nil {
		return fmt.Errorf("set consumer prefetch: %w", err)
	}
	deliveries, err := channel.ConsumeWithContext(ctx, queue.Name, "audit-log", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("start audit consumer: %w", err)
	}

	var workers sync.WaitGroup
	workers.Add(config.Concurrency)
	for workerIndex := 0; workerIndex < config.Concurrency; workerIndex++ {
		go func() {
			defer workers.Done()
			for delivery := range deliveries {
				if delivery.MessageId == "" {
					logger.Error("discarding audit event without message ID")
					_ = delivery.Nack(false, false)
					continue
				}
				processed, consumeErr := consumer.Consume(ctx, delivery.MessageId, delivery.Body)
				if consumeErr != nil {
					logger.Error("consume audit event", "event_id", delivery.MessageId, "error", consumeErr)
					select {
					case <-ctx.Done():
						_ = delivery.Nack(false, true)
						return
					case <-time.After(250 * time.Millisecond):
						_ = delivery.Nack(false, true)
					}
					continue
				}
				if err := delivery.Ack(false); err != nil {
					logger.Error("ack audit event", "event_id", delivery.MessageId, "error", err)
				} else {
					logger.Info("audit event handled", "event_id", delivery.MessageId, "processed", processed)
				}
			}
		}()
	}
	workers.Wait()
	return ctx.Err()
}
