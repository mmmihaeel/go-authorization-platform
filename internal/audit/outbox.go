package audit

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type OutboxMessage struct {
	ID         string
	EventType  string
	RoutingKey string
	Payload    []byte
	Attempts   int
	CreatedAt  time.Time
}

type OutboxRepository interface {
	ClaimPending(context.Context, int) ([]OutboxMessage, error)
	MarkPublished(context.Context, string) error
	MarkFailed(context.Context, string, error, time.Time) error
}

type Publisher interface {
	Publish(context.Context, OutboxMessage) error
}

type WorkerConfig struct {
	BatchSize    int
	Concurrency  int
	PollInterval time.Duration
	RetryBase    time.Duration
	OnError      func(error)
}

type Worker struct {
	repository OutboxRepository
	publisher  Publisher
	config     WorkerConfig
}

func NewWorker(repository OutboxRepository, publisher Publisher, config WorkerConfig) (*Worker, error) {
	if repository == nil {
		return nil, errors.New("outbox repository is required")
	}
	if publisher == nil {
		return nil, errors.New("publisher is required")
	}
	if config.BatchSize <= 0 {
		config.BatchSize = 50
	}
	if config.Concurrency <= 0 {
		config.Concurrency = 4
	}
	if config.PollInterval <= 0 {
		config.PollInterval = time.Second
	}
	if config.RetryBase <= 0 {
		config.RetryBase = time.Second
	}
	return &Worker{repository: repository, publisher: publisher, config: config}, nil
}

func (w *Worker) Run(ctx context.Context) error {
	ticker := time.NewTicker(w.config.PollInterval)
	defer ticker.Stop()
	for {
		if _, err := w.RunOnce(ctx); err != nil && !errors.Is(err, context.Canceled) && w.config.OnError != nil {
			w.config.OnError(err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (w *Worker) RunOnce(ctx context.Context) (int, error) {
	messages, err := w.repository.ClaimPending(ctx, w.config.BatchSize)
	if err != nil {
		return 0, err
	}
	if len(messages) == 0 {
		return 0, nil
	}

	semaphore := make(chan struct{}, w.config.Concurrency)
	errorsChannel := make(chan error, len(messages))
	var waitGroup sync.WaitGroup

	for _, message := range messages {
		message := message
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				errorsChannel <- ctx.Err()
				return
			}

			if err := w.publisher.Publish(ctx, message); err != nil {
				retryAt := time.Now().Add(w.retryDelay(message.Attempts + 1))
				if markErr := w.repository.MarkFailed(ctx, message.ID, err, retryAt); markErr != nil {
					errorsChannel <- fmt.Errorf("publish %s: %v; mark failed: %w", message.ID, err, markErr)
					return
				}
				errorsChannel <- fmt.Errorf("publish %s: %w", message.ID, err)
				return
			}
			if err := w.repository.MarkPublished(ctx, message.ID); err != nil {
				errorsChannel <- fmt.Errorf("mark %s published: %w", message.ID, err)
			}
		}()
	}

	waitGroup.Wait()
	close(errorsChannel)
	var combined error
	for workerErr := range errorsChannel {
		combined = errors.Join(combined, workerErr)
	}
	return len(messages), combined
}

func (w *Worker) retryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 8 {
		attempt = 8
	}
	return w.config.RetryBase * time.Duration(1<<(attempt-1))
}
