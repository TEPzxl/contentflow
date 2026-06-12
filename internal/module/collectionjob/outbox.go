package collectionjob

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/tepzxl/contentflow/internal/module/collector"
)

const (
	OutboxStatusPending    = "pending"
	OutboxStatusProcessing = "processing"
	OutboxStatusSent       = "sent"
	OutboxStatusFailed     = "failed"
)

type OutboxEvent struct {
	ID            int64
	Topic         string
	Key           string
	Value         []byte
	Status        string
	Attempts      int
	NextAttemptAt time.Time
	LastError     string
	ClaimID       string
	LockedUntil   time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
	SentAt        *time.Time
}

type CreateOutboxEventParams struct {
	Topic     string
	Key       string
	Payload   any
	CreatedAt time.Time
}

type OutboxRepository interface {
	Create(ctx context.Context, params CreateOutboxEventParams) (*OutboxEvent, error)
	ListReady(ctx context.Context, now time.Time, limit int) ([]OutboxEvent, int64, error)
	ClaimReady(ctx context.Context, now time.Time, limit int, claimID string, lockedUntil time.Time) ([]OutboxEvent, int64, error)
	FindByID(ctx context.Context, id int64) (*OutboxEvent, error)
	MarkSent(ctx context.Context, id int64, claimID string, sentAt time.Time) (*OutboxEvent, error)
	MarkFailed(ctx context.Context, id int64, claimID string, attempts int, nextAttemptAt time.Time, lastError string) (*OutboxEvent, error)
}

type OutboxDispatcher struct {
	repo      OutboxRepository
	writer    EventWriter
	logger    *slog.Logger
	observer  JobObserver
	now       func() time.Time
	batchSize int
	backoff   time.Duration
	interval  time.Duration
	claimTTL  time.Duration
}

type OutboxDispatcherOption func(*OutboxDispatcher)

func WithOutboxNow(now func() time.Time) OutboxDispatcherOption {
	return func(d *OutboxDispatcher) {
		if now != nil {
			d.now = now
		}
	}
}

func WithOutboxBatchSize(batchSize int) OutboxDispatcherOption {
	return func(d *OutboxDispatcher) {
		if batchSize > 0 {
			d.batchSize = batchSize
		}
	}
}

func WithOutboxBackoff(backoff time.Duration) OutboxDispatcherOption {
	return func(d *OutboxDispatcher) {
		if backoff > 0 {
			d.backoff = backoff
		}
	}
}

func WithOutboxInterval(interval time.Duration) OutboxDispatcherOption {
	return func(d *OutboxDispatcher) {
		if interval > 0 {
			d.interval = interval
		}
	}
}

func WithOutboxClaimTTL(claimTTL time.Duration) OutboxDispatcherOption {
	return func(d *OutboxDispatcher) {
		if claimTTL > 0 {
			d.claimTTL = claimTTL
		}
	}
}

func WithOutboxLogger(logger *slog.Logger) OutboxDispatcherOption {
	return func(d *OutboxDispatcher) {
		if logger != nil {
			d.logger = logger
		}
	}
}

func WithOutboxObserver(observer JobObserver) OutboxDispatcherOption {
	return func(d *OutboxDispatcher) {
		d.observer = observer
	}
}

func NewOutboxDispatcher(repo OutboxRepository, writer EventWriter, opts ...OutboxDispatcherOption) *OutboxDispatcher {
	d := &OutboxDispatcher{
		repo:      repo,
		writer:    writer,
		logger:    slog.Default(),
		now:       func() time.Time { return time.Now().UTC() },
		batchSize: 100,
		backoff:   time.Minute,
		interval:  time.Second,
		claimTTL:  5 * time.Minute,
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

func (d *OutboxDispatcher) Run(ctx context.Context) error {
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	for {
		if err := d.DispatchReady(ctx); err != nil {
			d.logger.Warn("dispatch outbox events failed", slog.String("error", err.Error()))
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (d *OutboxDispatcher) DispatchReady(ctx context.Context) error {
	now := d.now()
	claimID := randomTaskID()
	events, _, err := d.repo.ClaimReady(ctx, now, d.batchSize, claimID, now.Add(d.claimTTL))
	if err != nil {
		return fmt.Errorf("claim ready outbox events: %w", err)
	}

	for _, outboxEvent := range events {
		event := Event{
			Topic: outboxEvent.Topic,
			Key:   []byte(outboxEvent.Key),
			Value: outboxEvent.Value,
		}
		if err := d.writer.Write(ctx, event); err != nil {
			d.observeJob(ctx, outboxEvent.Topic, "failed")
			nextAttempts := outboxEvent.Attempts + 1
			nextAttemptAt := now.Add(d.retryBackoff(nextAttempts))
			if _, markErr := d.repo.MarkFailed(ctx, outboxEvent.ID, claimID, nextAttempts, nextAttemptAt, err.Error()); markErr != nil {
				return fmt.Errorf("mark outbox failed: %w", markErr)
			}
			continue
		}
		d.observeJob(ctx, outboxEvent.Topic, "success")

		if _, err := d.repo.MarkSent(ctx, outboxEvent.ID, claimID, now); err != nil {
			return fmt.Errorf("mark outbox sent: %w", err)
		}
	}

	return nil
}

func (d *OutboxDispatcher) observeJob(ctx context.Context, topic string, status string) {
	if d.observer == nil {
		return
	}
	d.observer.ObserveJob(ctx, JobObservation{
		Topic:  topic,
		Status: status,
	})
}

func (d *OutboxDispatcher) retryBackoff(attempts int) time.Duration {
	if attempts <= 1 {
		return d.backoff
	}
	backoff := d.backoff
	for i := 1; i < attempts; i++ {
		backoff *= 2
	}
	return backoff
}

type OutboxProducer struct {
	repo        OutboxRepository
	now         func() time.Time
	idGenerator func() string
}

type OutboxProducerOption func(*OutboxProducer)

func WithOutboxProducerNow(now func() time.Time) OutboxProducerOption {
	return func(p *OutboxProducer) {
		if now != nil {
			p.now = now
		}
	}
}

func WithOutboxProducerIDGenerator(idGenerator func() string) OutboxProducerOption {
	return func(p *OutboxProducer) {
		if idGenerator != nil {
			p.idGenerator = idGenerator
		}
	}
}

func NewOutboxProducer(repo OutboxRepository, opts ...OutboxProducerOption) *OutboxProducer {
	p := &OutboxProducer{
		repo:        repo,
		now:         func() time.Time { return time.Now().UTC() },
		idGenerator: randomTaskID,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *OutboxProducer) RequestCollection(ctx context.Context, req collector.CollectSourceRequest) (*collector.RequestCollectionResponse, error) {
	taskID := p.idGenerator()
	key := idempotencyKey(req.SourceID)
	payload := CollectionRequested{
		TaskID:         taskID,
		UserID:         req.UserID,
		SourceID:       req.SourceID,
		IdempotencyKey: key,
		Attempt:        0,
		RequestedAt:    p.now(),
	}
	outboxEvent, err := p.repo.Create(ctx, CreateOutboxEventParams{
		Topic:     TopicCollectionRequested,
		Key:       key,
		Payload:   payload,
		CreatedAt: p.now(),
	})
	if err != nil {
		return nil, fmt.Errorf("create collection requested outbox event: %w", err)
	}
	if outboxEvent != nil {
		var queued CollectionRequested
		if err := json.Unmarshal(outboxEvent.Value, &queued); err != nil {
			return nil, fmt.Errorf("decode collection requested outbox event: %w", err)
		}
		if queued.TaskID != "" {
			taskID = queued.TaskID
		}
	}

	return &collector.RequestCollectionResponse{
		TaskID:   taskID,
		SourceID: req.SourceID,
		Status:   collector.RunStatusQueued,
	}, nil
}

func (p *OutboxProducer) CollectSource(ctx context.Context, req collector.CollectSourceRequest) (*collector.CollectSourceResponse, error) {
	result, err := p.RequestCollection(ctx, req)
	if err != nil {
		return nil, err
	}
	return &collector.CollectSourceResponse{
		SourceID: result.SourceID,
		Status:   result.Status,
	}, nil
}

func marshalOutboxPayload(payload any) ([]byte, error) {
	value, ok := payload.([]byte)
	if ok {
		return value, nil
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal outbox payload: %w", err)
	}
	return data, nil
}
