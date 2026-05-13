package collectionjob

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/tepzxl/contentflow/internal/module/collector"
)

var ErrInvalidMessage = errors.New("invalid message")

type EventReader interface {
	Read(ctx context.Context) (Message, error)
	Commit(ctx context.Context, msg Message) error
	Close() error
}

type CollectionService interface {
	CollectSource(ctx context.Context, req collector.CollectSourceRequest) (*collector.CollectSourceResponse, error)
}

type Worker struct {
	reader      EventReader
	writer      EventWriter
	service     CollectionService
	logger      *slog.Logger
	now         func() time.Time
	maxAttempts int
}

type WorkerOption func(*Worker)

func WithWorkerNow(now func() time.Time) WorkerOption {
	return func(w *Worker) {
		if now != nil {
			w.now = now
		}
	}
}

func WithMaxAttempts(maxAttempts int) WorkerOption {
	return func(w *Worker) {
		if maxAttempts > 0 {
			w.maxAttempts = maxAttempts
		}
	}
}

func WithWorkerLogger(logger *slog.Logger) WorkerOption {
	return func(w *Worker) {
		if logger != nil {
			w.logger = logger
		}
	}
}

func NewWorker(reader EventReader, writer EventWriter, service CollectionService, opts ...WorkerOption) *Worker {
	w := &Worker{
		reader:      reader,
		writer:      writer,
		service:     service,
		logger:      slog.Default(),
		now:         func() time.Time { return time.Now().UTC() },
		maxAttempts: 3,
	}

	for _, opt := range opts {
		opt(w)
	}

	return w
}

func (w *Worker) Run(ctx context.Context) error {
	for {
		msg, err := w.reader.Read(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return fmt.Errorf("read collection job: %w", err)
		}

		if err := w.HandleMessage(ctx, msg); err != nil {
			w.logger.Warn("handle collection job failed", slog.String("error", err.Error()))
		}

		if err := w.reader.Commit(ctx, msg); err != nil {
			return fmt.Errorf("commit collection job: %w", err)
		}
	}
}

func (w *Worker) HandleMessage(ctx context.Context, msg Message) error {
	var event CollectionRequested
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidMessage, err)
	}

	resp, err := w.service.CollectSource(ctx, collector.CollectSourceRequest{
		UserID:   event.UserID,
		SourceID: event.SourceID,
	})
	if err != nil {
		return w.handleFailed(ctx, event, err)
	}

	if resp == nil {
		return nil
	}

	completed := CollectionCompleted{
		TaskID:            event.TaskID,
		SourceID:          event.SourceID,
		RunID:             resp.RunID,
		FetchedCount:      resp.FetchedCount,
		InsertedCount:     resp.InsertedCount,
		DuplicatedCount:   resp.DuplicatedCount,
		CompletedAt:       w.now(),
		IdempotencyKey:    event.IdempotencyKey,
		OriginalRequested: event.RequestedAt,
	}

	return w.writeJSON(ctx, TopicCollectionCompleted, event.IdempotencyKey, completed)
}

func (w *Worker) handleFailed(ctx context.Context, event CollectionRequested, cause error) error {
	failed := CollectionFailed{
		TaskID:            event.TaskID,
		SourceID:          event.SourceID,
		Attempt:           event.Attempt,
		ErrorMessage:      cause.Error(),
		FailedAt:          w.now(),
		IdempotencyKey:    event.IdempotencyKey,
		OriginalRequested: event.RequestedAt,
	}
	if err := w.writeJSON(ctx, TopicCollectionFailed, event.IdempotencyKey, failed); err != nil {
		return err
	}

	nextAttempt := event.Attempt + 1
	if nextAttempt >= w.maxAttempts {
		return w.writeJSON(ctx, TopicCollectionDLQ, event.IdempotencyKey, event)
	}

	event.Attempt = nextAttempt
	return w.writeJSON(ctx, TopicCollectionRequested, event.IdempotencyKey, event)
}

func (w *Worker) writeJSON(ctx context.Context, topic string, key string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal %s event: %w", topic, err)
	}

	if err := w.writer.Write(ctx, Event{
		Topic: topic,
		Key:   []byte(key),
		Value: data,
	}); err != nil {
		return fmt.Errorf("write %s event: %w", topic, err)
	}

	return nil
}
