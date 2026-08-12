package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/mmmihaeel/go-authorization-platform/internal/audit"
	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQPublisher struct {
	url        string
	connection *amqp.Connection
	channel    *amqp.Channel
	exchange   string
	mu         sync.Mutex
}

func NewRabbitMQPublisher(url, exchange string) (*RabbitMQPublisher, error) {
	if url == "" {
		return nil, fmt.Errorf("RabbitMQ URL is required")
	}
	if exchange == "" {
		exchange = "authorization.audit"
	}
	publisher := &RabbitMQPublisher{url: url, exchange: exchange}
	if err := publisher.connectLocked(); err != nil {
		return nil, err
	}
	return publisher, nil
}

func (p *RabbitMQPublisher) Publish(ctx context.Context, message audit.OutboxMessage) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.channel == nil || p.channel.IsClosed() || p.connection == nil || p.connection.IsClosed() {
		if err := p.connectLocked(); err != nil {
			return err
		}
	}

	headers := amqp.Table{"event_type": message.EventType}
	if !json.Valid(message.Payload) {
		headers["payload_format"] = "binary"
	}
	confirmation, err := p.channel.PublishWithDeferredConfirmWithContext(ctx, p.exchange, message.RoutingKey, true, false, amqp.Publishing{
		Headers: headers, ContentType: "application/json", DeliveryMode: amqp.Persistent,
		MessageId: message.ID, Type: message.EventType, Timestamp: time.Now().UTC(), Body: message.Payload,
	})
	if err != nil {
		p.closeLocked()
		return fmt.Errorf("publish RabbitMQ message: %w", err)
	}
	acknowledged, err := confirmation.WaitContext(ctx)
	if err != nil {
		p.closeLocked()
		return fmt.Errorf("wait for RabbitMQ publisher confirm: %w", err)
	}
	if !acknowledged {
		p.closeLocked()
		return fmt.Errorf("RabbitMQ negatively acknowledged message %s", message.ID)
	}
	return nil
}

func (p *RabbitMQPublisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.closeLocked()
}

func (p *RabbitMQPublisher) connectLocked() error {
	_ = p.closeLocked()
	connection, err := amqp.Dial(p.url)
	if err != nil {
		return fmt.Errorf("connect RabbitMQ: %w", err)
	}
	channel, err := connection.Channel()
	if err != nil {
		_ = connection.Close()
		return fmt.Errorf("open RabbitMQ channel: %w", err)
	}
	if err := channel.ExchangeDeclare(p.exchange, "topic", true, false, false, false, nil); err != nil {
		_ = channel.Close()
		_ = connection.Close()
		return fmt.Errorf("declare RabbitMQ exchange: %w", err)
	}
	if err := channel.Confirm(false); err != nil {
		_ = channel.Close()
		_ = connection.Close()
		return fmt.Errorf("enable RabbitMQ publisher confirms: %w", err)
	}
	p.connection = connection
	p.channel = channel
	return nil
}

func (p *RabbitMQPublisher) closeLocked() error {
	if p.channel != nil && !p.channel.IsClosed() {
		_ = p.channel.Close()
	}
	p.channel = nil
	if p.connection != nil && !p.connection.IsClosed() {
		err := p.connection.Close()
		p.connection = nil
		return err
	}
	p.connection = nil
	return nil
}
