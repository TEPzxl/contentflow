package collectionjob

import "time"

const (
	TopicCollectionRequested = "collection.requested"
	TopicCollectionCompleted = "collection.completed"
	TopicCollectionFailed    = "collection.failed"
	TopicCollectionDLQ       = "collection.dlq"
)

type Event struct {
	Topic string
	Key   []byte
	Value []byte
}

type Message struct {
	Topic    string
	Key      []byte
	Value    []byte
	metadata any
}

type CollectionRequested struct {
	TaskID         string    `json:"task_id"`
	UserID         int64     `json:"user_id"`
	SourceID       int64     `json:"source_id"`
	IdempotencyKey string    `json:"idempotency_key"`
	Attempt        int       `json:"attempt"`
	RequestedAt    time.Time `json:"requested_at"`
	NextAttemptAt  time.Time `json:"next_attempt_at,omitempty"`
}

type CollectionCompleted struct {
	TaskID            string    `json:"task_id"`
	SourceID          int64     `json:"source_id"`
	RunID             int64     `json:"run_id"`
	FetchedCount      int       `json:"fetched_count"`
	InsertedCount     int       `json:"inserted_count"`
	DuplicatedCount   int       `json:"duplicated_count"`
	CompletedAt       time.Time `json:"completed_at"`
	IdempotencyKey    string    `json:"idempotency_key"`
	OriginalRequested time.Time `json:"original_requested_at"`
}

type CollectionFailed struct {
	TaskID            string    `json:"task_id"`
	SourceID          int64     `json:"source_id"`
	Attempt           int       `json:"attempt"`
	ErrorMessage      string    `json:"error_message"`
	FailedAt          time.Time `json:"failed_at"`
	IdempotencyKey    string    `json:"idempotency_key"`
	OriginalRequested time.Time `json:"original_requested_at"`
}
