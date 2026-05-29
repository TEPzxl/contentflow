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

func TestWorker_RunDoesNotCommitFailedHandling(t *testing.T) {
	now := time.Date(2026, 5, 13, 14, 0, 0, 0, time.UTC)
	reader := &fakeEventReader{messages: []Message{{
		Topic: TopicCollectionRequested,
		Key:   []byte("collection:source:42"),
		Value: marshalJSON(t, CollectionRequested{
			TaskID:         "task-commit",
			UserID:         100,
			SourceID:       42,
			IdempotencyKey: "collection:source:42",
			RequestedAt:    now,
		}),
	}}}
	worker := NewWorker(
		reader,
		&fakeEventWriter{err: errors.New("kafka unavailable")},
		&fakeCollectionService{resp: &collector.CollectSourceResponse{RunID: 11, SourceID: 42, Status: collector.RunStatusSuccess}},
		WithWorkerNow(func() time.Time { return now }),
	)

	if err := worker.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if reader.commits != 0 {
		t.Fatalf("commits = %d, want 0", reader.commits)
	}
}

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
				if !retry.NextAttemptAt.Equal(now.Add(time.Minute)) {
					t.Fatalf("retry next attempt = %v, want %v", retry.NextAttemptAt, now.Add(time.Minute))
				}
			},
		},
		{
			name: "permanent failure writes dlq without retry",
			event: CollectionRequested{
				TaskID:         "task-4",
				UserID:         100,
				SourceID:       42,
				IdempotencyKey: "collection:source:42",
				Attempt:        0,
				RequestedAt:    now,
			},
			service: &fakeCollectionService{
				err: PermanentError(errCollect),
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

func TestWorker_HandleMessage_skipsCompletedTask(t *testing.T) {
	now := time.Date(2026, 5, 13, 14, 0, 0, 0, time.UTC)
	repo := newMemoryJobExecutionRepository()
	if err := repo.seed(JobExecution{
		TaskID:         "task-duplicate",
		IdempotencyKey: "collection:source:42",
		SourceID:       42,
		Status:         JobExecutionStatusSucceeded,
		RunID:          11,
		CreatedAt:      now,
		UpdatedAt:      now,
	}); err != nil {
		t.Fatalf("seed execution: %v", err)
	}
	writer := &fakeEventWriter{}
	service := &fakeCollectionService{}
	worker := NewWorker(
		nil,
		writer,
		service,
		WithWorkerNow(func() time.Time { return now }),
		WithJobExecutionRepository(repo),
	)

	err := worker.HandleMessage(context.Background(), Message{
		Topic: TopicCollectionRequested,
		Key:   []byte("collection:source:42"),
		Value: marshalJSON(t, CollectionRequested{
			TaskID:         "task-duplicate",
			UserID:         100,
			SourceID:       42,
			IdempotencyKey: "collection:source:42",
			RequestedAt:    now,
		}),
	})
	if err != nil {
		t.Fatalf("HandleMessage() error = %v", err)
	}
	if repo.claims != 1 {
		t.Fatalf("claim calls = %d, want 1", repo.claims)
	}
	if len(service.reqs) != 0 {
		t.Fatalf("service calls = %d, want 0", len(service.reqs))
	}
	if len(writer.events) != 0 {
		t.Fatalf("written events = %d, want 0", len(writer.events))
	}
}

func TestWorker_HandleMessage_waitsUntilNextAttempt(t *testing.T) {
	now := time.Date(2026, 5, 13, 14, 0, 0, 0, time.UTC)
	event := CollectionRequested{
		TaskID:         "task-delayed",
		UserID:         100,
		SourceID:       42,
		IdempotencyKey: "collection:source:42",
		Attempt:        1,
		RequestedAt:    now.Add(-time.Minute),
		NextAttemptAt:  now.Add(2 * time.Minute),
	}
	service := &fakeCollectionService{
		resp: &collector.CollectSourceResponse{
			RunID:    11,
			SourceID: 42,
			Status:   collector.RunStatusSuccess,
		},
	}
	var slept time.Duration
	worker := NewWorker(
		nil,
		&fakeEventWriter{},
		service,
		WithWorkerNow(func() time.Time { return now }),
		WithWorkerSleep(func(ctx context.Context, d time.Duration) error {
			slept = d
			return nil
		}),
	)

	err := worker.HandleMessage(context.Background(), Message{
		Topic: TopicCollectionRequested,
		Key:   []byte(event.IdempotencyKey),
		Value: marshalJSON(t, event),
	})
	if err != nil {
		t.Fatalf("HandleMessage() error = %v", err)
	}
	if slept != 2*time.Minute {
		t.Fatalf("slept = %v, want %v", slept, 2*time.Minute)
	}
	if len(service.reqs) != 1 {
		t.Fatalf("service calls = %d, want 1", len(service.reqs))
	}
}

func TestWorker_HandleMessage_persistsDLQItem(t *testing.T) {
	now := time.Date(2026, 5, 13, 14, 0, 0, 0, time.UTC)
	repo := newMemoryDLQRepository()
	event := CollectionRequested{
		TaskID:         "task-dlq",
		UserID:         100,
		SourceID:       42,
		IdempotencyKey: "collection:source:42",
		Attempt:        2,
		RequestedAt:    now.Add(-time.Minute),
	}
	worker := NewWorker(
		nil,
		&fakeEventWriter{},
		&fakeCollectionService{err: errCollect},
		WithMaxAttempts(3),
		WithWorkerNow(func() time.Time { return now }),
		WithDLQRepository(repo),
	)

	err := worker.HandleMessage(context.Background(), Message{
		Topic: TopicCollectionRequested,
		Key:   []byte(event.IdempotencyKey),
		Value: marshalJSON(t, event),
	})
	if err != nil {
		t.Fatalf("HandleMessage() error = %v", err)
	}

	items, total, err := repo.List(context.Background(), ListDLQItemsParams{Status: DLQStatusPending})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("dlq total/items = %d/%d, want 1/1", total, len(items))
	}
	if items[0].TaskID != "task-dlq" || items[0].ErrorMessage != errCollect.Error() {
		t.Fatalf("dlq item = %#v", items[0])
	}
}

type fakeEventReader struct {
	messages []Message
	commits  int
	reads    int
}

func (r *fakeEventReader) Read(ctx context.Context) (Message, error) {
	if r.reads >= len(r.messages) {
		return Message{}, context.Canceled
	}
	msg := r.messages[r.reads]
	r.reads++
	return msg, nil
}

func (r *fakeEventReader) Commit(ctx context.Context, msg Message) error {
	r.commits++
	return nil
}

func (r *fakeEventReader) Close() error {
	return nil
}

type memoryJobExecutionRepository struct {
	executions map[string]JobExecution
	claims     int
}

func newMemoryJobExecutionRepository() *memoryJobExecutionRepository {
	return &memoryJobExecutionRepository{executions: map[string]JobExecution{}}
}

func (r *memoryJobExecutionRepository) seed(execution JobExecution) error {
	if execution.TaskID == "" {
		return ErrInvalidMessage
	}
	r.executions[execution.TaskID] = execution
	return nil
}

func (r *memoryJobExecutionRepository) Claim(ctx context.Context, event CollectionRequested, claimID string, now time.Time, lockedUntil time.Time) (*JobExecution, bool, error) {
	r.claims++
	execution, ok := r.executions[event.TaskID]
	if ok && execution.Status == JobExecutionStatusSucceeded {
		return &execution, false, nil
	}
	execution = JobExecution{
		TaskID:         event.TaskID,
		IdempotencyKey: event.IdempotencyKey,
		SourceID:       event.SourceID,
		Status:         JobExecutionStatusProcessing,
		Attempt:        event.Attempt,
		ClaimID:        claimID,
		LockedUntil:    lockedUntil,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	r.executions[event.TaskID] = execution
	return &execution, true, nil
}

func (r *memoryJobExecutionRepository) MarkSucceeded(ctx context.Context, taskID string, claimID string, runID int64, now time.Time) (*JobExecution, error) {
	execution := r.executions[taskID]
	execution.Status = JobExecutionStatusSucceeded
	execution.RunID = runID
	execution.ClaimID = ""
	execution.LockedUntil = time.Time{}
	execution.UpdatedAt = now
	r.executions[taskID] = execution
	return &execution, nil
}

func (r *memoryJobExecutionRepository) MarkFailed(ctx context.Context, taskID string, claimID string, attempt int, errMessage string, now time.Time) (*JobExecution, error) {
	execution := r.executions[taskID]
	execution.Status = JobExecutionStatusFailed
	execution.Attempt = attempt
	execution.LastError = errMessage
	execution.ClaimID = ""
	execution.LockedUntil = time.Time{}
	execution.UpdatedAt = now
	r.executions[taskID] = execution
	return &execution, nil
}

func (r *memoryJobExecutionRepository) MarkDLQ(ctx context.Context, taskID string, claimID string, errMessage string, now time.Time) (*JobExecution, error) {
	execution := r.executions[taskID]
	execution.Status = JobExecutionStatusDLQ
	execution.LastError = errMessage
	execution.ClaimID = ""
	execution.LockedUntil = time.Time{}
	execution.UpdatedAt = now
	r.executions[taskID] = execution
	return &execution, nil
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
