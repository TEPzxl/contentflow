package collectionjob

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
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

type JobObservation struct {
	Topic  string
	Status string
}

type JobObserver interface {
	ObserveJob(ctx context.Context, observation JobObservation)
}

type Worker struct {
	reader        EventReader
	writer        EventWriter
	service       CollectionService
	logger        *slog.Logger
	observer      JobObserver
	dlqRepo       DLQRepository
	executionRepo JobExecutionRepository
	now           func() time.Time
	sleep         func(ctx context.Context, d time.Duration) error
	maxAttempts   int
	backoffBase   time.Duration
	claimTTL      time.Duration
}

type WorkerOption func(*Worker)

func WithWorkerNow(now func() time.Time) WorkerOption {
	return func(w *Worker) {
		if now != nil {
			w.now = now
		}
	}
}

func WithWorkerSleep(sleep func(ctx context.Context, d time.Duration) error) WorkerOption {
	return func(w *Worker) {
		if sleep != nil {
			w.sleep = sleep
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

func WithRetryBackoff(base time.Duration) WorkerOption {
	return func(w *Worker) {
		if base > 0 {
			w.backoffBase = base
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

func WithJobObserver(observer JobObserver) WorkerOption {
	return func(w *Worker) {
		w.observer = observer
	}
}

func WithDLQRepository(repo DLQRepository) WorkerOption {
	return func(w *Worker) {
		w.dlqRepo = repo
	}
}

func WithJobExecutionRepository(repo JobExecutionRepository) WorkerOption {
	return func(w *Worker) {
		w.executionRepo = repo
	}
}

func NewWorker(reader EventReader, writer EventWriter, service CollectionService, opts ...WorkerOption) *Worker {
	w := &Worker{
		reader:      reader,
		writer:      writer,
		service:     service,
		logger:      slog.Default(),
		now:         func() time.Time { return time.Now().UTC() },
		sleep:       sleepContext,
		maxAttempts: 3,
		backoffBase: time.Minute,
		claimTTL:    10 * time.Minute,
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
			if !errors.Is(err, ErrInvalidMessage) {
				continue
			}
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
	if err := validateCollectionRequestedMessage(msg, event); err != nil {
		return err
	}

	if err := w.waitUntilNextAttempt(ctx, event); err != nil {
		return err
	}

	claimID, shouldProcess, err := w.claimExecution(ctx, event)
	if err != nil {
		return err
	}
	if !shouldProcess {
		return nil
	}

	resp, err := w.service.CollectSource(ctx, collector.CollectSourceRequest{
		UserID:   event.UserID,
		SourceID: event.SourceID,
	})
	if err != nil {
		return w.handleFailed(ctx, event, claimID, err)
	}

	if resp == nil {
		return w.markExecutionSucceeded(ctx, event.TaskID, claimID, 0)
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

	if err := w.writeJSON(ctx, TopicCollectionCompleted, event.IdempotencyKey, completed); err != nil {
		return err
	}
	return w.markExecutionSucceeded(ctx, event.TaskID, claimID, resp.RunID)
}

func validateCollectionRequestedMessage(msg Message, event CollectionRequested) error {
	if msg.Topic != "" && msg.Topic != TopicCollectionRequested {
		return fmt.Errorf("%w: unexpected topic %q", ErrInvalidMessage, msg.Topic)
	}
	if strings.TrimSpace(event.TaskID) == "" {
		return fmt.Errorf("%w: missing task_id", ErrInvalidMessage)
	}
	if event.UserID <= 0 {
		return fmt.Errorf("%w: invalid user_id", ErrInvalidMessage)
	}
	if event.SourceID <= 0 {
		return fmt.Errorf("%w: invalid source_id", ErrInvalidMessage)
	}
	if strings.TrimSpace(event.IdempotencyKey) == "" {
		return fmt.Errorf("%w: missing idempotency_key", ErrInvalidMessage)
	}
	if event.Attempt < 0 {
		return fmt.Errorf("%w: invalid attempt", ErrInvalidMessage)
	}
	if event.RequestedAt.IsZero() {
		return fmt.Errorf("%w: missing requested_at", ErrInvalidMessage)
	}
	if len(msg.Key) > 0 && string(msg.Key) != event.IdempotencyKey {
		return fmt.Errorf("%w: message key does not match idempotency_key", ErrInvalidMessage)
	}
	return nil
}

func (w *Worker) claimExecution(ctx context.Context, event CollectionRequested) (string, bool, error) {
	if w.executionRepo == nil {
		return "", true, nil
	}
	now := w.now()
	claimID := randomTaskID()
	_, shouldProcess, err := w.executionRepo.Claim(ctx, event, claimID, now, now.Add(w.claimTTL))
	if err != nil {
		return "", false, fmt.Errorf("claim collection job execution: %w", err)
	}
	return claimID, shouldProcess, nil
}

func (w *Worker) markExecutionSucceeded(ctx context.Context, taskID string, claimID string, runID int64) error {
	if w.executionRepo == nil {
		return nil
	}
	if _, err := w.executionRepo.MarkSucceeded(ctx, taskID, claimID, runID, w.now()); err != nil {
		return fmt.Errorf("mark collection job execution succeeded: %w", err)
	}
	return nil
}

func (w *Worker) handleFailed(ctx context.Context, event CollectionRequested, claimID string, cause error) error {
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
	if IsPermanentError(cause) || nextAttempt >= w.maxAttempts {
		if err := w.writeDLQ(ctx, event, cause); err != nil {
			return err
		}
		return w.markExecutionDLQ(ctx, event.TaskID, claimID, cause)
	}

	event.Attempt = nextAttempt
	event.NextAttemptAt = w.now().Add(w.retryBackoff(nextAttempt))
	if err := w.writeJSON(ctx, TopicCollectionRequested, event.IdempotencyKey, event); err != nil {
		return err
	}
	return w.markExecutionFailed(ctx, event.TaskID, claimID, event.Attempt-1, cause)
}

func (w *Worker) markExecutionFailed(ctx context.Context, taskID string, claimID string, attempt int, cause error) error {
	if w.executionRepo == nil {
		return nil
	}
	if _, err := w.executionRepo.MarkFailed(ctx, taskID, claimID, attempt, cause.Error(), w.now()); err != nil {
		return fmt.Errorf("mark collection job execution failed: %w", err)
	}
	return nil
}

func (w *Worker) markExecutionDLQ(ctx context.Context, taskID string, claimID string, cause error) error {
	if w.executionRepo == nil {
		return nil
	}
	if _, err := w.executionRepo.MarkDLQ(ctx, taskID, claimID, cause.Error(), w.now()); err != nil {
		return fmt.Errorf("mark collection job execution dlq: %w", err)
	}
	return nil
}

func (w *Worker) writeDLQ(ctx context.Context, event CollectionRequested, cause error) error {
	if w.dlqRepo != nil {
		if _, err := w.dlqRepo.Create(ctx, CreateDLQItemParams{
			Event:        event,
			ErrorMessage: cause.Error(),
			CreatedAt:    w.now(),
		}); err != nil {
			return fmt.Errorf("create dlq item: %w", err)
		}
	}

	return w.writeJSON(ctx, TopicCollectionDLQ, event.IdempotencyKey, event)
}

func (w *Worker) waitUntilNextAttempt(ctx context.Context, event CollectionRequested) error {
	if event.NextAttemptAt.IsZero() {
		return nil
	}

	delay := event.NextAttemptAt.Sub(w.now())
	if delay <= 0 {
		return nil
	}

	if err := w.sleep(ctx, delay); err != nil {
		return fmt.Errorf("wait for next collection attempt: %w", err)
	}
	return nil
}

func (w *Worker) retryBackoff(attempt int) time.Duration {
	if attempt <= 1 {
		return w.backoffBase
	}

	backoff := w.backoffBase
	for i := 1; i < attempt; i++ {
		backoff *= 2
	}
	return backoff
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
		w.observeJob(ctx, topic, "failed")
		return fmt.Errorf("write %s event: %w", topic, err)
	}

	w.observeJob(ctx, topic, "success")
	return nil
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (w *Worker) observeJob(ctx context.Context, topic string, status string) {
	if w.observer == nil {
		return
	}
	w.observer.ObserveJob(ctx, JobObservation{
		Topic:  topic,
		Status: status,
	})
}
