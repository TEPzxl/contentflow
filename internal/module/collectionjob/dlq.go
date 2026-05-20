package collectionjob

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

const (
	DLQStatusPending  = "pending"
	DLQStatusReplayed = "replayed"
	DLQStatusHandled  = "handled"
)

type DLQItem struct {
	ID             int64
	TaskID         string
	UserID         int64
	SourceID       int64
	IdempotencyKey string
	Attempt        int
	ErrorMessage   string
	Payload        CollectionRequested
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	ReplayedAt     *time.Time
	HandledAt      *time.Time
}

type CreateDLQItemParams struct {
	Event        CollectionRequested
	ErrorMessage string
	CreatedAt    time.Time
}

type ListDLQItemsParams struct {
	Status string
	Limit  int
	Offset int
}

type DLQRepository interface {
	Create(ctx context.Context, params CreateDLQItemParams) (*DLQItem, error)
	List(ctx context.Context, params ListDLQItemsParams) ([]DLQItem, int64, error)
	FindByID(ctx context.Context, id int64) (*DLQItem, error)
	MarkReplayed(ctx context.Context, id int64, replayedAt time.Time) (*DLQItem, error)
	MarkHandled(ctx context.Context, id int64, handledAt time.Time) (*DLQItem, error)
}

type ListDLQItemsRequest struct {
	Status string
	Limit  int
	Offset int
}

type ListDLQItemsResponse struct {
	Items  []DLQItemDTO
	Total  int64
	Limit  int
	Offset int
}

type ReplayDLQItemRequest struct {
	ID int64
}

type ReplayDLQItemResponse struct {
	Item DLQItemDTO
}

type MarkDLQHandledRequest struct {
	ID int64
}

type MarkDLQHandledResponse struct {
	Item DLQItemDTO
}

type DLQItemDTO struct {
	ID             int64
	TaskID         string
	UserID         int64
	SourceID       int64
	IdempotencyKey string
	Attempt        int
	ErrorMessage   string
	Status         string
	CreatedAt      string
	UpdatedAt      string
	ReplayedAt     *string
	HandledAt      *string
}

type DLQService struct {
	repo   DLQRepository
	writer EventWriter
	now    func() time.Time
}

type DLQOption func(*DLQService)

func WithDLQNow(now func() time.Time) DLQOption {
	return func(s *DLQService) {
		if now != nil {
			s.now = now
		}
	}
}

func NewDLQService(repo DLQRepository, writer EventWriter, opts ...DLQOption) *DLQService {
	s := &DLQService{
		repo:   repo,
		writer: writer,
		now:    func() time.Time { return time.Now().UTC() },
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *DLQService) List(ctx context.Context, req ListDLQItemsRequest) (*ListDLQItemsResponse, int64, error) {
	limit := normalizeLimit(req.Limit)
	offset := normalizeOffset(req.Offset)
	items, total, err := s.repo.List(ctx, ListDLQItemsParams{
		Status: req.Status,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list dlq items: %w", err)
	}
	return &ListDLQItemsResponse{
		Items:  toDLQItemDTOs(items),
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}, total, nil
}

func (s *DLQService) Replay(ctx context.Context, req ReplayDLQItemRequest) (*ReplayDLQItemResponse, error) {
	item, err := s.repo.FindByID(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("find dlq item: %w", err)
	}

	event := item.Payload
	event.Attempt = 0
	event.NextAttemptAt = time.Time{}
	data, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("marshal replay event: %w", err)
	}
	if err := s.writer.Write(ctx, Event{
		Topic: TopicCollectionRequested,
		Key:   []byte(event.IdempotencyKey),
		Value: data,
	}); err != nil {
		return nil, fmt.Errorf("write replay event: %w", err)
	}

	updated, err := s.repo.MarkReplayed(ctx, item.ID, s.now())
	if err != nil {
		return nil, fmt.Errorf("mark dlq replayed: %w", err)
	}
	return &ReplayDLQItemResponse{Item: toDLQItemDTO(*updated)}, nil
}

func (s *DLQService) MarkHandled(ctx context.Context, req MarkDLQHandledRequest) (*MarkDLQHandledResponse, error) {
	updated, err := s.repo.MarkHandled(ctx, req.ID, s.now())
	if err != nil {
		return nil, fmt.Errorf("mark dlq handled: %w", err)
	}
	return &MarkDLQHandledResponse{Item: toDLQItemDTO(*updated)}, nil
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func normalizeOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

func toDLQItemDTOs(items []DLQItem) []DLQItemDTO {
	dtos := make([]DLQItemDTO, 0, len(items))
	for _, item := range items {
		dtos = append(dtos, toDLQItemDTO(item))
	}
	return dtos
}

func toDLQItemDTO(item DLQItem) DLQItemDTO {
	return DLQItemDTO{
		ID:             item.ID,
		TaskID:         item.TaskID,
		UserID:         item.UserID,
		SourceID:       item.SourceID,
		IdempotencyKey: item.IdempotencyKey,
		Attempt:        item.Attempt,
		ErrorMessage:   item.ErrorMessage,
		Status:         item.Status,
		CreatedAt:      item.CreatedAt.Format(time.RFC3339Nano),
		UpdatedAt:      item.UpdatedAt.Format(time.RFC3339Nano),
		ReplayedAt:     formatOptionalTime(item.ReplayedAt),
		HandledAt:      formatOptionalTime(item.HandledAt),
	}
}

func formatOptionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.Format(time.RFC3339Nano)
	return &formatted
}
