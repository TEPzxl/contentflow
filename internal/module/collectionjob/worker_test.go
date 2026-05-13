package collectionjob

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/tepzxl/contentflow/internal/module/collector"
)

var errCollect = errors.New("collect failed")

func TestWorker_HandleMessage(t *testing.T) {
	now := time.Date(2026, 5, 13, 14, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		event   CollectionRequested
		service *fakeCollectionService
		options []WorkerOption
		want    func(t *testing.T, writer *fakeEventWriter)
		wantErr error
	}{
		{
			name: "success writes completed event",
			event: CollectionRequested{
				TaskID:         "task-1",
				UserID:         100,
				SourceID:       42,
				IdempotencyKey: "collection:source:42",
				RequestedAt:    now,
			},
			service: &fakeCollectionService{
				resp: &collector.CollectSourceResponse{
					RunID:           11,
					SourceID:        42,
					Status:          collector.RunStatusSuccess,
					FetchedCount:    3,
					InsertedCount:   2,
					DuplicatedCount: 1,
				},
			},
			want: func(t *testing.T, writer *fakeEventWriter) {
				t.Helper()
				if len(writer.events) != 1 {
					t.Fatalf("event count = %d, want 1", len(writer.events))
				}
				if writer.events[0].Topic != TopicCollectionCompleted {
					t.Fatalf("topic = %q, want %q", writer.events[0].Topic, TopicCollectionCompleted)
				}

				var payload CollectionCompleted
				unmarshalEvent(t, writer.events[0], &payload)
				if payload.TaskID != "task-1" || payload.RunID != 11 || payload.InsertedCount != 2 {
					t.Fatalf("payload = %#v", payload)
				}
			},
		},
		{
			name: "failure before max attempts requeues request and writes failed event",
			event: CollectionRequested{
				TaskID:         "task-2",
				UserID:         100,
				SourceID:       42,
				IdempotencyKey: "collection:source:42",
				Attempt:        0,
				RequestedAt:    now,
			},
			service: &fakeCollectionService{
				err: errCollect,
			},
			options: []WorkerOption{
				WithMaxAttempts(3),
				WithWorkerNow(func() time.Time { return now }),
			},
			want: func(t *testing.T, writer *fakeEventWriter) {
				t.Helper()
				if len(writer.events) != 2 {
					t.Fatalf("event count = %d, want 2", len(writer.events))
				}
				if writer.events[0].Topic != TopicCollectionFailed {
					t.Fatalf("first topic = %q", writer.events[0].Topic)
				}
				if writer.events[1].Topic != TopicCollectionRequested {
					t.Fatalf("second topic = %q", writer.events[1].Topic)
				}

				var retry CollectionRequested
				unmarshalEvent(t, writer.events[1], &retry)
				if retry.Attempt != 1 {
					t.Fatalf("retry attempt = %d, want 1", retry.Attempt)
				}
			},
		},
		{
			name: "failure at max attempts writes dlq",
			event: CollectionRequested{
				TaskID:         "task-3",
				UserID:         100,
				SourceID:       42,
				IdempotencyKey: "collection:source:42",
				Attempt:        2,
				RequestedAt:    now,
			},
			service: &fakeCollectionService{
				err: errCollect,
			},
			options: []WorkerOption{
				WithMaxAttempts(3),
				WithWorkerNow(func() time.Time { return now }),
			},
			want: func(t *testing.T, writer *fakeEventWriter) {
				t.Helper()
				if len(writer.events) != 2 {
					t.Fatalf("event count = %d, want 2", len(writer.events))
				}
				if writer.events[0].Topic != TopicCollectionFailed {
					t.Fatalf("first topic = %q", writer.events[0].Topic)
				}
				if writer.events[1].Topic != TopicCollectionDLQ {
					t.Fatalf("second topic = %q", writer.events[1].Topic)
				}
			},
		},
		{
			name:    "invalid json returns error",
			event:   CollectionRequested{},
			service: &fakeCollectionService{},
			wantErr: ErrInvalidMessage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer := &fakeEventWriter{}
			options := append([]WorkerOption{WithWorkerNow(func() time.Time { return now })}, tt.options...)
			worker := NewWorker(nil, writer, tt.service, options...)

			var msg Message
			if tt.name == "invalid json returns error" {
				msg = Message{Topic: TopicCollectionRequested, Key: []byte("bad"), Value: []byte(`{`)}
			} else {
				msg = Message{
					Topic: TopicCollectionRequested,
					Key:   []byte(tt.event.IdempotencyKey),
					Value: marshalJSON(t, tt.event),
				}
			}

			err := worker.HandleMessage(context.Background(), msg)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("HandleMessage() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("HandleMessage() unexpected error = %v", err)
			}
			if tt.want != nil {
				tt.want(t, writer)
			}
		})
	}
}

type fakeCollectionService struct {
	resp *collector.CollectSourceResponse
	err  error
	reqs []collector.CollectSourceRequest
}

func (s *fakeCollectionService) CollectSource(ctx context.Context, req collector.CollectSourceRequest) (*collector.CollectSourceResponse, error) {
	s.reqs = append(s.reqs, req)
	if s.err != nil {
		return nil, s.err
	}
	return s.resp, nil
}

func marshalJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return data
}

func unmarshalEvent(t *testing.T, event Event, out any) {
	t.Helper()
	if err := json.Unmarshal(event.Value, out); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}
}
