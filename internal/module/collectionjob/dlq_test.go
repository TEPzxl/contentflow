package collectionjob

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDLQService_ListReplayAndMarkHandled(t *testing.T) {
	now := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
	repo := newMemoryDLQRepository()
	writer := &fakeEventWriter{}
	service := NewDLQService(
		repo,
		writer,
		WithDLQNow(func() time.Time { return now }),
		WithDLQTaskIDGenerator(func() string { return "task-replay-1" }),
	)
	ctx := context.Background()

	event := CollectionRequested{
		TaskID:         "task-1",
		UserID:         100,
		SourceID:       42,
		IdempotencyKey: "collection:source:42",
		Attempt:        2,
		RequestedAt:    now.Add(-time.Minute),
	}
	item, err := repo.Create(ctx, CreateDLQItemParams{
		Event:        event,
		ErrorMessage: "collect failed",
		CreatedAt:    now,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	list, total, err := service.List(ctx, ListDLQItemsRequest{UserID: 100, Status: DLQStatusPending, Limit: 20})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if total != 1 || len(list.Items) != 1 {
		t.Fatalf("list total/items = %d/%d, want 1/1", total, len(list.Items))
	}
	if list.Items[0].TaskID != "task-1" || list.Items[0].ErrorMessage != "collect failed" {
		t.Fatalf("list item = %#v", list.Items[0])
	}

	replayed, err := service.Replay(ctx, ReplayDLQItemRequest{UserID: 100, ID: item.ID})
	if err != nil {
		t.Fatalf("Replay() error = %v", err)
	}
	if replayed.Item.Status != DLQStatusReplayed {
		t.Fatalf("replayed status = %s, want %s", replayed.Item.Status, DLQStatusReplayed)
	}
	if len(writer.events) != 1 {
		t.Fatalf("written events = %d, want 1", len(writer.events))
	}
	if writer.events[0].Topic != TopicCollectionRequested {
		t.Fatalf("topic = %s, want %s", writer.events[0].Topic, TopicCollectionRequested)
	}
	var replayEvent CollectionRequested
	unmarshalEvent(t, writer.events[0], &replayEvent)
	if replayEvent.TaskID != "task-replay-1" {
		t.Fatalf("replay task id = %q, want new replay task id", replayEvent.TaskID)
	}
	if replayEvent.TaskID == event.TaskID {
		t.Fatalf("replay task id reused original task id %q", event.TaskID)
	}
	if replayEvent.Attempt != 0 || !replayEvent.NextAttemptAt.IsZero() {
		t.Fatalf("replay event = %#v, want reset attempt", replayEvent)
	}

	handled, err := service.MarkHandled(ctx, MarkDLQHandledRequest{UserID: 100, ID: item.ID})
	if err != nil {
		t.Fatalf("MarkHandled() error = %v", err)
	}
	if handled.Item.Status != DLQStatusHandled {
		t.Fatalf("handled status = %s, want %s", handled.Item.Status, DLQStatusHandled)
	}
}

func TestDLQService_ScopesItemsByUser(t *testing.T) {
	now := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
	repo := newMemoryDLQRepository()
	writer := &fakeEventWriter{}
	service := NewDLQService(repo, writer, WithDLQNow(func() time.Time { return now }))
	ctx := context.Background()

	own, err := repo.Create(ctx, CreateDLQItemParams{
		Event: CollectionRequested{
			TaskID:         "own-task",
			UserID:         100,
			SourceID:       42,
			IdempotencyKey: "collection:source:42",
			Attempt:        2,
			RequestedAt:    now,
		},
		ErrorMessage: "own failure",
		CreatedAt:    now,
	})
	if err != nil {
		t.Fatalf("Create() own error = %v", err)
	}
	other, err := repo.Create(ctx, CreateDLQItemParams{
		Event: CollectionRequested{
			TaskID:         "other-task",
			UserID:         200,
			SourceID:       99,
			IdempotencyKey: "collection:source:99",
			Attempt:        2,
			RequestedAt:    now,
		},
		ErrorMessage: "other failure",
		CreatedAt:    now,
	})
	if err != nil {
		t.Fatalf("Create() other error = %v", err)
	}

	list, total, err := service.List(ctx, ListDLQItemsRequest{UserID: 100, Status: DLQStatusPending, Limit: 20})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if total != 1 || len(list.Items) != 1 || list.Items[0].ID != own.ID {
		t.Fatalf("scoped list total/items = %d/%#v, want only own item %d", total, list.Items, own.ID)
	}

	if _, err := service.Replay(ctx, ReplayDLQItemRequest{UserID: 100, ID: other.ID}); !errors.Is(err, ErrDLQItemNotFound) {
		t.Fatalf("Replay() other user error = %v, want ErrDLQItemNotFound", err)
	}
	if len(writer.events) != 0 {
		t.Fatalf("written events = %d, want 0", len(writer.events))
	}

	if _, err := service.MarkHandled(ctx, MarkDLQHandledRequest{UserID: 100, ID: other.ID}); !errors.Is(err, ErrDLQItemNotFound) {
		t.Fatalf("MarkHandled() other user error = %v, want ErrDLQItemNotFound", err)
	}
}

type memoryDLQRepository struct {
	nextID int64
	items  map[int64]DLQItem
}

func newMemoryDLQRepository() *memoryDLQRepository {
	return &memoryDLQRepository{
		nextID: 1,
		items:  map[int64]DLQItem{},
	}
}

func (r *memoryDLQRepository) Create(_ context.Context, params CreateDLQItemParams) (*DLQItem, error) {
	now := params.CreatedAt
	item := DLQItem{
		ID:             r.nextID,
		TaskID:         params.Event.TaskID,
		UserID:         params.Event.UserID,
		SourceID:       params.Event.SourceID,
		IdempotencyKey: params.Event.IdempotencyKey,
		Attempt:        params.Event.Attempt,
		ErrorMessage:   params.ErrorMessage,
		Payload:        params.Event,
		Status:         DLQStatusPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	r.items[item.ID] = item
	r.nextID++
	return &item, nil
}

func (r *memoryDLQRepository) List(_ context.Context, params ListDLQItemsParams) ([]DLQItem, int64, error) {
	var items []DLQItem
	for _, item := range r.items {
		if params.UserID > 0 && item.UserID != params.UserID {
			continue
		}
		if params.Status == "" || item.Status == params.Status {
			items = append(items, item)
		}
	}
	return items, int64(len(items)), nil
}

func (r *memoryDLQRepository) FindByUserIDAndID(_ context.Context, userID, id int64) (*DLQItem, error) {
	item, ok := r.items[id]
	if !ok || item.UserID != userID {
		return nil, ErrDLQItemNotFound
	}
	return &item, nil
}

func (r *memoryDLQRepository) FindByID(ctx context.Context, id int64) (*DLQItem, error) {
	return r.FindByUserIDAndID(ctx, r.items[id].UserID, id)
}

func (r *memoryDLQRepository) MarkReplayed(_ context.Context, id int64, replayedAt time.Time) (*DLQItem, error) {
	item, ok := r.items[id]
	if !ok {
		return nil, errors.New("dlq item not found")
	}
	item.Status = DLQStatusReplayed
	item.ReplayedAt = &replayedAt
	item.UpdatedAt = replayedAt
	r.items[id] = item
	return &item, nil
}

func (r *memoryDLQRepository) MarkHandled(_ context.Context, id int64, handledAt time.Time) (*DLQItem, error) {
	item, ok := r.items[id]
	if !ok {
		return nil, errors.New("dlq item not found")
	}
	item.Status = DLQStatusHandled
	item.HandledAt = &handledAt
	item.UpdatedAt = handledAt
	r.items[id] = item
	return &item, nil
}
