package collectionjob

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/tepzxl/contentflow/internal/module/collector"
)

func TestProducer_RequestCollection(t *testing.T) {
	now := time.Date(2026, 5, 13, 13, 0, 0, 0, time.UTC)
	writer := &fakeEventWriter{}
	producer := NewProducer(
		writer,
		WithNow(func() time.Time { return now }),
		WithIDGenerator(func() string { return "task-1" }),
	)

	resp, err := producer.RequestCollection(context.Background(), collector.CollectSourceRequest{
		UserID:   100,
		SourceID: 42,
	})
	if err != nil {
		t.Fatalf("RequestCollection() error = %v", err)
	}

	if resp.TaskID != "task-1" {
		t.Fatalf("TaskID = %q, want task-1", resp.TaskID)
	}
	if resp.SourceID != 42 {
		t.Fatalf("SourceID = %d, want 42", resp.SourceID)
	}
	if resp.Status != collector.RunStatusQueued {
		t.Fatalf("Status = %q, want queued", resp.Status)
	}

	if len(writer.events) != 1 {
		t.Fatalf("event count = %d, want 1", len(writer.events))
	}

	event := writer.events[0]
	if event.Topic != TopicCollectionRequested {
		t.Fatalf("topic = %q, want %q", event.Topic, TopicCollectionRequested)
	}
	if string(event.Key) != "collection:source:42" {
		t.Fatalf("key = %q, want collection:source:42", string(event.Key))
	}

	var payload CollectionRequested
	if err := json.Unmarshal(event.Value, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	if payload.TaskID != "task-1" || payload.UserID != 100 || payload.SourceID != 42 {
		t.Fatalf("payload = %#v", payload)
	}
	if payload.IdempotencyKey != "collection:source:42" {
		t.Fatalf("IdempotencyKey = %q", payload.IdempotencyKey)
	}
	if !payload.RequestedAt.Equal(now) {
		t.Fatalf("RequestedAt = %v, want %v", payload.RequestedAt, now)
	}
}

func TestProducer_CollectSourceCompatibility(t *testing.T) {
	writer := &fakeEventWriter{}
	producer := NewProducer(
		writer,
		WithIDGenerator(func() string { return "task-2" }),
	)

	resp, err := producer.CollectSource(context.Background(), collector.CollectSourceRequest{
		UserID:   100,
		SourceID: 42,
	})
	if err != nil {
		t.Fatalf("CollectSource() error = %v", err)
	}

	if resp.Status != collector.RunStatusQueued {
		t.Fatalf("Status = %q, want queued", resp.Status)
	}
	if resp.SourceID != 42 {
		t.Fatalf("SourceID = %d, want 42", resp.SourceID)
	}
	if len(writer.events) != 1 {
		t.Fatalf("event count = %d, want 1", len(writer.events))
	}
}

type fakeEventWriter struct {
	events []Event
	err    error
}

func (w *fakeEventWriter) Write(ctx context.Context, event Event) error {
	if w.err != nil {
		return w.err
	}
	w.events = append(w.events, event)
	return nil
}
