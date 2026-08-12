//go:build rabbitmq

package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mmmihaeel/go-authorization-platform/internal/audit"
	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQPublisher struct {
	connection *amqp.Connection
	channel    *amqp.Channel
	exchange   string
}

func NewRabbitMQPublisher(url, exchange string) (*RabbitMQPublisher, error) {
	connection, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("connect RabbitMQ: %w", err)
	}
	channel, err := connection.Channel()
	if err != nil {
		_ = connection.Close()
		return nil, fmt.Errorf("open RabbitMQ channel: %w", err)
	}
	if exchange == "" {
		exchange = "authorization.audit"
	}
	if err := channel.ExchangeDeclare(exchange, "topic", true, false, false, false, nil); err != nil {
		_ = channel.Close()
		_ = connection.Close()
		return nil, fmt.Errorf("declare RabbitMQ exchange: %w", err)
	}
	return &RabbitMQPublisher{connection: connection, channel: channel, exchange: exchange}, nil
}

func (p *RabbitMQPublisher) Publish(ctx context.Context, message audit.OutboxMessage) error {
	headers := amqp.Table{"event_type": message.EventType}
	if !json.Valid(message.Payload) {
		headers["payload_format"] = "binary"
	}
	return p.channel.PublishWithContext(ctx, p.exchange, message.RoutingKey, false, false, amqp.Publishing{
		Headers: headers, ContentType: "application/json", DeliveryMode: amqp.Persistent,
		MessageId: message.ID, Timestamp: time.Now().UTC(), Body: message.Payload,
	})
}

func (p *RabbitMQPublisher) Close() error {
	if p.channel != nil {
		_ = p.channel.Close()
	}
	if p.connection != nil {
		return p.connection.Close()
	}
	return nil
}
