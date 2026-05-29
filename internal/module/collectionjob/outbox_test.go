package collectionjob

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tepzxl/contentflow/internal/module/collector"
)

func TestOutboxDispatcher_DispatchReadyMarksSentAndFailed(t *testing.T) {
	now := time.Date(2026, 5, 14, 11, 0, 0, 0, time.UTC)
	repo := newMemoryOutboxRepository()
	writer := &fakeEventWriter{}
	observer := &recordingJobObserver{}
	dispatcher := NewOutboxDispatcher(
		repo,
		writer,
		WithOutboxNow(func() time.Time { return now }),
		WithOutboxBatchSize(10),
		WithOutboxBackoff(time.Minute),
		WithOutboxObserver(observer),
	)
	ctx := context.Background()

	sent, err := repo.Create(ctx, CreateOutboxEventParams{
		Topic:     TopicCollectionRequested,
		Key:       "collection:source:42",
		Payload:   CollectionRequested{TaskID: "task-ok", IdempotencyKey: "collection:source:42"},
		CreatedAt: now,
	})
	if err != nil {
		t.Fatalf("Create(sent) error = %v", err)
	}
	failed, err := repo.Create(ctx, CreateOutboxEventParams{
		Topic:     TopicCollectionRequested,
		Key:       "collection:source:43",
		Payload:   CollectionRequested{TaskID: "task-fail", IdempotencyKey: "collection:source:43"},
		CreatedAt: now,
	})
	if err != nil {
		t.Fatalf("Create(failed) error = %v", err)
	}
	writer.failKeys = map[string]error{"collection:source:43": errors.New("kafka unavailable")}

	err = dispatcher.DispatchReady(ctx)
	if err != nil {
		t.Fatalf("DispatchReady() error = %v", err)
	}
	if len(writer.events) != 1 {
		t.Fatalf("written events = %d, want 1", len(writer.events))
	}

	sentItem, _ := repo.FindByID(ctx, sent.ID)
	if sentItem.Status != OutboxStatusSent || sentItem.SentAt == nil {
		t.Fatalf("sent item = %#v", sentItem)
	}

	failedItem, _ := repo.FindByID(ctx, failed.ID)
	if failedItem.Status != OutboxStatusFailed {
		t.Fatalf("failed status = %s, want %s", failedItem.Status, OutboxStatusFailed)
	}
	if failedItem.Attempts != 1 {
		t.Fatalf("failed attempts = %d, want 1", failedItem.Attempts)
	}
	if !failedItem.NextAttemptAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("next attempt = %v, want %v", failedItem.NextAttemptAt, now.Add(time.Minute))
	}
	if len(observer.observations) != 2 {
		t.Fatalf("observer calls = %d, want 2", len(observer.observations))
	}
}

func TestOutboxProducer_RequestCollectionWritesOutbox(t *testing.T) {
	now := time.Date(2026, 5, 14, 11, 0, 0, 0, time.UTC)
	repo := newMemoryOutboxRepository()
	producer := NewOutboxProducer(
		repo,
		WithOutboxProducerNow(func() time.Time { return now }),
		WithOutboxProducerIDGenerator(func() string { return "task-1" }),
	)

	resp, err := producer.RequestCollection(context.Background(), collector.CollectSourceRequest{
		UserID:   100,
		SourceID: 42,
	})
	if err != nil {
		t.Fatalf("RequestCollection() error = %v", err)
	}
	if resp.TaskID != "task-1" {
		t.Fatalf("TaskID = %s, want task-1", resp.TaskID)
	}

	events, total, err := repo.ListReady(context.Background(), now, 10)
	if err != nil {
		t.Fatalf("ListReady() error = %v", err)
	}
	if total != 1 || len(events) != 1 {
		t.Fatalf("outbox total/events = %d/%d, want 1/1", total, len(events))
	}
	if events[0].Topic != TopicCollectionRequested || events[0].Key != "collection:source:42" {
		t.Fatalf("outbox event = %#v", events[0])
	}
}

type memoryOutboxRepository struct {
	nextID int64
	events map[int64]OutboxEvent
}

func newMemoryOutboxRepository() *memoryOutboxRepository {
	return &memoryOutboxRepository{
		nextID: 1,
		events: map[int64]OutboxEvent{},
	}
}

func (r *memoryOutboxRepository) Create(_ context.Context, params CreateOutboxEventParams) (*OutboxEvent, error) {
	value, err := marshalOutboxPayload(params.Payload)
	if err != nil {
		return nil, err
	}
	now := params.CreatedAt
	event := OutboxEvent{
		ID:            r.nextID,
		Topic:         params.Topic,
		Key:           params.Key,
		Value:         value,
		Status:        OutboxStatusPending,
		NextAttemptAt: now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	r.events[event.ID] = event
	r.nextID++
	return &event, nil
}

func (r *memoryOutboxRepository) ListReady(_ context.Context, now time.Time, limit int) ([]OutboxEvent, int64, error) {
	var events []OutboxEvent
	for _, event := range r.events {
		if event.Status != OutboxStatusSent && event.Status != OutboxStatusProcessing && !event.NextAttemptAt.After(now) {
			events = append(events, event)
			if len(events) == limit {
				break
			}
		}
	}
	return events, int64(len(events)), nil
}

func (r *memoryOutboxRepository) ClaimReady(_ context.Context, now time.Time, limit int, claimID string, lockedUntil time.Time) ([]OutboxEvent, int64, error) {
	var events []OutboxEvent
	var total int64
	for id, event := range r.events {
		ready := (event.Status == OutboxStatusPending || event.Status == OutboxStatusFailed) && !event.NextAttemptAt.After(now)
		expired := event.Status == OutboxStatusProcessing && !event.LockedUntil.After(now)
		if !ready && !expired {
			continue
		}
		total++
		if len(events) == limit {
			continue
		}
		event.Status = OutboxStatusProcessing
		event.ClaimID = claimID
		event.LockedUntil = lockedUntil
		event.UpdatedAt = now
		r.events[id] = event
		events = append(events, event)
	}
	return events, total, nil
}

func (r *memoryOutboxRepository) FindByID(_ context.Context, id int64) (*OutboxEvent, error) {
	event := r.events[id]
	return &event, nil
}

func (r *memoryOutboxRepository) MarkSent(_ context.Context, id int64, claimID string, sentAt time.Time) (*OutboxEvent, error) {
	event := r.events[id]
	if event.Status != OutboxStatusProcessing || event.ClaimID != claimID {
		return nil, ErrOutboxClaimLost
	}
	event.Status = OutboxStatusSent
	event.SentAt = &sentAt
	event.UpdatedAt = sentAt
	event.ClaimID = ""
	event.LockedUntil = time.Time{}
	r.events[id] = event
	return &event, nil
}

func (r *memoryOutboxRepository) MarkFailed(_ context.Context, id int64, claimID string, attempts int, nextAttemptAt time.Time, lastError string) (*OutboxEvent, error) {
	event := r.events[id]
	if event.Status != OutboxStatusProcessing || event.ClaimID != claimID {
		return nil, ErrOutboxClaimLost
	}
	event.Status = OutboxStatusFailed
	event.Attempts = attempts
	event.NextAttemptAt = nextAttemptAt
	event.LastError = lastError
	event.UpdatedAt = nextAttemptAt
	event.ClaimID = ""
	event.LockedUntil = time.Time{}
	r.events[id] = event
	return &event, nil
}
