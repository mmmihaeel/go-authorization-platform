package events

import (
	"context"
	"errors"
)

type InboxClaim uint8

const (
	ClaimAcquired InboxClaim = iota + 1
	ClaimCompleted
	ClaimBusy
)

var ErrEventInProgress = errors.New("event is already being processed")

type Inbox interface {
	TryBegin(context.Context, string) (InboxClaim, error)
	Complete(context.Context, string) error
	Release(context.Context, string) error
}

type Handler interface {
	Handle(context.Context, []byte) error
}

type Consumer struct {
	inbox   Inbox
	handler Handler
}

func NewConsumer(inbox Inbox, handler Handler) (*Consumer, error) {
	if inbox == nil {
		return nil, errors.New("inbox is required")
	}
	if handler == nil {
		return nil, errors.New("handler is required")
	}
	return &Consumer{inbox: inbox, handler: handler}, nil
}

func (c *Consumer) Consume(ctx context.Context, eventID string, payload []byte) (processed bool, err error) {
	claim, err := c.inbox.TryBegin(ctx, eventID)
	if err != nil {
		return false, err
	}
	switch claim {
	case ClaimCompleted:
		return false, nil
	case ClaimBusy:
		return false, ErrEventInProgress
	case ClaimAcquired:
	default:
		return false, errors.New("inbox returned an invalid claim state")
	}
	if err := c.handler.Handle(ctx, payload); err != nil {
		_ = c.inbox.Release(ctx, eventID)
		return false, err
	}
	if err := c.inbox.Complete(ctx, eventID); err != nil {
		_ = c.inbox.Release(ctx, eventID)
		return false, err
	}
	return true, nil
}
