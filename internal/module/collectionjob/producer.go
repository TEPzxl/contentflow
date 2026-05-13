package collectionjob

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/tepzxl/contentflow/internal/module/collector"
)

type EventWriter interface {
	Write(ctx context.Context, event Event) error
}

type Producer struct {
	writer      EventWriter
	now         func() time.Time
	idGenerator func() string
}

type ProducerOption func(*Producer)

func WithNow(now func() time.Time) ProducerOption {
	return func(p *Producer) {
		if now != nil {
			p.now = now
		}
	}
}

func WithIDGenerator(idGenerator func() string) ProducerOption {
	return func(p *Producer) {
		if idGenerator != nil {
			p.idGenerator = idGenerator
		}
	}
}

func NewProducer(writer EventWriter, opts ...ProducerOption) *Producer {
	p := &Producer{
		writer:      writer,
		now:         func() time.Time { return time.Now().UTC() },
		idGenerator: randomTaskID,
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

func (p *Producer) RequestCollection(ctx context.Context, req collector.CollectSourceRequest) (*collector.RequestCollectionResponse, error) {
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

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal collection requested event: %w", err)
	}

	if err := p.writer.Write(ctx, Event{
		Topic: TopicCollectionRequested,
		Key:   []byte(key),
		Value: data,
	}); err != nil {
		return nil, fmt.Errorf("write collection requested event: %w", err)
	}

	return &collector.RequestCollectionResponse{
		TaskID:   taskID,
		SourceID: req.SourceID,
		Status:   collector.RunStatusQueued,
	}, nil
}

func (p *Producer) CollectSource(ctx context.Context, req collector.CollectSourceRequest) (*collector.CollectSourceResponse, error) {
	result, err := p.RequestCollection(ctx, req)
	if err != nil {
		return nil, err
	}

	return &collector.CollectSourceResponse{
		SourceID: result.SourceID,
		Status:   result.Status,
	}, nil
}

func idempotencyKey(sourceID int64) string {
	return fmt.Sprintf("collection:source:%d", sourceID)
}

func randomTaskID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("task-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
