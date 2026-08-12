package audit

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type memoryOutbox struct {
	mu        sync.Mutex
	messages  []OutboxMessage
	published map[string]bool
	failed    map[string]bool
}

func (m *memoryOutbox) ClaimPending(_ context.Context, limit int) ([]OutboxMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if limit > len(m.messages) {
		limit = len(m.messages)
	}
	result := append([]OutboxMessage(nil), m.messages[:limit]...)
	m.messages = m.messages[limit:]
	return result, nil
}
func (m *memoryOutbox) MarkPublished(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.published[id] = true
	return nil
}
func (m *memoryOutbox) MarkFailed(_ context.Context, id string, _ error, _ time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failed[id] = true
	return nil
}

type publisherFunc func(context.Context, OutboxMessage) error

func (f publisherFunc) Publish(ctx context.Context, message OutboxMessage) error {
	return f(ctx, message)
}

func TestWorkerPublishesBatchConcurrently(t *testing.T) {
	repository := &memoryOutbox{
		messages:  []OutboxMessage{{ID: "1"}, {ID: "2"}, {ID: "3"}},
		published: map[string]bool{}, failed: map[string]bool{},
	}
	worker, err := NewWorker(repository, publisherFunc(func(context.Context, OutboxMessage) error { return nil }), WorkerConfig{Concurrency: 2})
	if err != nil {
		t.Fatal(err)
	}
	count, err := worker.RunOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 || len(repository.published) != 3 {
		t.Fatalf("count=%d published=%v", count, repository.published)
	}
}

func TestWorkerBoundsPublishConcurrency(t *testing.T) {
	repository := &memoryOutbox{
		messages:  []OutboxMessage{{ID: "1"}, {ID: "2"}, {ID: "3"}, {ID: "4"}, {ID: "5"}},
		published: map[string]bool{}, failed: map[string]bool{},
	}
	var mutex sync.Mutex
	inFlight, maximum := 0, 0
	worker, err := NewWorker(repository, publisherFunc(func(context.Context, OutboxMessage) error {
		mutex.Lock()
		inFlight++
		if inFlight > maximum {
			maximum = inFlight
		}
		mutex.Unlock()
		time.Sleep(10 * time.Millisecond)
		mutex.Lock()
		inFlight--
		mutex.Unlock()
		return nil
	}), WorkerConfig{Concurrency: 2})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := worker.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if maximum != 2 {
		t.Fatalf("maximum concurrency=%d, want 2", maximum)
	}
}

func TestWorkerMarksPublishFailureForRetry(t *testing.T) {
	repository := &memoryOutbox{
		messages:  []OutboxMessage{{ID: "1"}},
		published: map[string]bool{}, failed: map[string]bool{},
	}
	worker, _ := NewWorker(repository, publisherFunc(func(context.Context, OutboxMessage) error { return errors.New("broker unavailable") }), WorkerConfig{})
	count, err := worker.RunOnce(context.Background())
	if count != 1 || err == nil || !repository.failed["1"] {
		t.Fatalf("count=%d err=%v failed=%v", count, err, repository.failed)
	}
}
