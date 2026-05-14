package collectionjob

import (
	"context"
	"testing"
	"time"

	"github.com/tepzxl/contentflow/internal/module/collector"
)

type recordingJobObserver struct {
	observations []JobObservation
}

func (o *recordingJobObserver) ObserveJob(_ context.Context, observation JobObservation) {
	o.observations = append(o.observations, observation)
}

func TestWorker_ObserverReceivesWrittenJobEvent(t *testing.T) {
	now := time.Date(2026, 5, 13, 14, 0, 0, 0, time.UTC)
	writer := &fakeEventWriter{}
	observer := &recordingJobObserver{}
	service := &fakeCollectionService{
		resp: &collector.CollectSourceResponse{
			RunID:         11,
			SourceID:      42,
			Status:        collector.RunStatusSuccess,
			FetchedCount:  1,
			InsertedCount: 1,
		},
	}
	worker := NewWorker(
		nil,
		writer,
		service,
		WithWorkerNow(func() time.Time { return now }),
		WithJobObserver(observer),
	)

	event := CollectionRequested{
		TaskID:         "task-1",
		UserID:         100,
		SourceID:       42,
		IdempotencyKey: "collection:source:42",
		RequestedAt:    now,
	}

	err := worker.HandleMessage(context.Background(), Message{
		Topic: TopicCollectionRequested,
		Key:   []byte(event.IdempotencyKey),
		Value: marshalJSON(t, event),
	})
	if err != nil {
		t.Fatalf("HandleMessage() error = %v", err)
	}

	if len(observer.observations) != 1 {
		t.Fatalf("observer calls = %d, want 1", len(observer.observations))
	}

	got := observer.observations[0]
	if got.Topic != TopicCollectionCompleted {
		t.Fatalf("Topic = %s, want %s", got.Topic, TopicCollectionCompleted)
	}
	if got.Status != "success" {
		t.Fatalf("Status = %s, want success", got.Status)
	}
}
