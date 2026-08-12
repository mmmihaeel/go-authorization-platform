package events

import (
	"context"
	"testing"
)

type handlerFunc func(context.Context, []byte) error

func (f handlerFunc) Handle(ctx context.Context, payload []byte) error { return f(ctx, payload) }

func TestConsumerProcessesEventOnlyOnce(t *testing.T) {
	calls := 0
	consumer, err := NewConsumer(NewMemoryInbox(), handlerFunc(func(context.Context, []byte) error { calls++; return nil }))
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
