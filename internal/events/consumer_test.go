package events

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

type handlerFunc func(context.Context, []byte) error

func (f handlerFunc) Handle(ctx context.Context, payload []byte) error { return f(ctx, payload) }

type memoryInbox struct {
	mu    sync.Mutex
	state map[string]string
}

func newMemoryInbox() *memoryInbox { return &memoryInbox{state: map[string]string{}} }

func (i *memoryInbox) TryBegin(_ context.Context, id string) (InboxClaim, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	switch i.state[id] {
	case "completed":
		return ClaimCompleted, nil
	case "processing":
		return ClaimBusy, nil
	}
	i.state[id] = "processing"
	return ClaimAcquired, nil
}

func (i *memoryInbox) Complete(_ context.Context, id string) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.state[id] = "completed"
	return nil
}

func (i *memoryInbox) Release(_ context.Context, id string) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	delete(i.state, id)
	return nil
}

func TestConsumerProcessesEventOnlyOnce(t *testing.T) {
	calls := 0
	consumer, err := NewConsumer(newMemoryInbox(), handlerFunc(func(context.Context, []byte) error { calls++; return nil }))
	if err != nil {
		t.Fatal(err)
	}
	processed, err := consumer.Consume(context.Background(), "event-1", []byte("payload"))
	if err != nil || !processed {
		t.Fatalf("processed=%v err=%v", processed, err)
	}
	processed, err = consumer.Consume(context.Background(), "event-1", []byte("payload"))
	if err != nil || processed || calls != 1 {
		t.Fatalf("processed=%v err=%v calls=%d", processed, err, calls)
	}
}

func TestConsumerConcurrentDuplicateIsHandledOnce(t *testing.T) {
	var calls atomic.Int32
	consumer, err := NewConsumer(newMemoryInbox(), handlerFunc(func(context.Context, []byte) error {
		calls.Add(1)
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	var waitGroup sync.WaitGroup
	for index := 0; index < 20; index++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			_, _ = consumer.Consume(context.Background(), "event-1", []byte("payload"))
		}()
	}
	waitGroup.Wait()
	if calls.Load() != 1 {
		t.Fatalf("handler calls=%d, want 1", calls.Load())
	}
}

func TestConsumerDistinguishesBusyFromCompleted(t *testing.T) {
	inbox := newMemoryInbox()
	if claim, err := inbox.TryBegin(context.Background(), "event-1"); err != nil || claim != ClaimAcquired {
		t.Fatalf("claim=%v err=%v", claim, err)
	}
	consumer, err := NewConsumer(inbox, handlerFunc(func(context.Context, []byte) error { return nil }))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := consumer.Consume(context.Background(), "event-1", nil); !errors.Is(err, ErrEventInProgress) {
		t.Fatalf("busy error=%v, want %v", err, ErrEventInProgress)
	}
	if err := inbox.Complete(context.Background(), "event-1"); err != nil {
		t.Fatal(err)
	}
	if processed, err := consumer.Consume(context.Background(), "event-1", nil); err != nil || processed {
		t.Fatalf("completed event processed=%v err=%v", processed, err)
	}
}
