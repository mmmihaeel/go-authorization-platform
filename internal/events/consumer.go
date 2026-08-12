package events

import (
	"context"
	"errors"
	"sync"
)

type Inbox interface {
	TryBegin(context.Context, string) (bool, error)
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
	acquired, err := c.inbox.TryBegin(ctx, eventID)
	if err != nil || !acquired {
		return false, err
	}
	if err := c.handler.Handle(ctx, payload); err != nil {
		_ = c.inbox.Release(ctx, eventID)
		return false, err
	}
	if err := c.inbox.Complete(ctx, eventID); err != nil {
		return false, err
	}
	return true, nil
}

type MemoryInbox struct {
	mu    sync.Mutex
	state map[string]string
}

func NewMemoryInbox() *MemoryInbox { return &MemoryInbox{state: map[string]string{}} }

func (i *MemoryInbox) TryBegin(_ context.Context, id string) (bool, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if _, exists := i.state[id]; exists {
		return false, nil
	}
	i.state[id] = "processing"
	return true, nil
}
func (i *MemoryInbox) Complete(_ context.Context, id string) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.state[id] = "completed"
	return nil
}
func (i *MemoryInbox) Release(_ context.Context, id string) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	delete(i.state, id)
	return nil
}
