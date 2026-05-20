package collectionjob

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var ErrOutboxEventNotFound = errors.New("outbox event not found")

type OutboxEventModel struct {
	ID            int64          `gorm:"column:id;primaryKey"`
	Topic         string         `gorm:"column:topic;type:varchar(200);not null;index"`
	EventKey      string         `gorm:"column:event_key;type:varchar(300);not null;index"`
	PayloadJSON   datatypes.JSON `gorm:"column:payload_json;type:jsonb;not null"`
	Status        string         `gorm:"column:status;type:varchar(50);not null;index"`
	Attempts      int            `gorm:"column:attempts;not null"`
	NextAttemptAt time.Time      `gorm:"column:next_attempt_at;not null;index"`
	LastError     string         `gorm:"column:last_error;type:text;not null"`
	CreatedAt     time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt     time.Time      `gorm:"column:updated_at;not null"`
	SentAt        *time.Time     `gorm:"column:sent_at"`
}

func (OutboxEventModel) TableName() string {
	return "outbox_events"
}

type GormOutboxRepository struct {
	db *gorm.DB
}

func NewGormOutboxRepository(db *gorm.DB) OutboxRepository {
	return &GormOutboxRepository{db: db}
}

func (r *GormOutboxRepository) Create(ctx context.Context, params CreateOutboxEventParams) (*OutboxEvent, error) {
	payload, err := marshalOutboxPayload(params.Payload)
	if err != nil {
		return nil, err
	}
	now := params.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	model := OutboxEventModel{
		Topic:         params.Topic,
		EventKey:      params.Key,
		PayloadJSON:   datatypes.JSON(payload),
		Status:        OutboxStatusPending,
		Attempts:      0,
		NextAttemptAt: now,
		LastError:     "",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return nil, fmt.Errorf("create outbox event: %w", err)
	}
	event := outboxModelToEvent(model)
	return &event, nil
}

func (r *GormOutboxRepository) ListReady(ctx context.Context, now time.Time, limit int) ([]OutboxEvent, int64, error) {
	if limit <= 0 {
		limit = 100
	}
	query := r.db.WithContext(ctx).
		Model(&OutboxEventModel{}).
		Where("status IN ?", []string{OutboxStatusPending, OutboxStatusFailed}).
		Where("next_attempt_at <= ?", now)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count ready outbox events: %w", err)
	}

	var models []OutboxEventModel
	if err := query.
		Order("next_attempt_at ASC").
		Order("id ASC").
		Limit(limit).
		Find(&models).Error; err != nil {
		return nil, 0, fmt.Errorf("list ready outbox events: %w", err)
	}

	events := make([]OutboxEvent, 0, len(models))
	for _, model := range models {
		events = append(events, outboxModelToEvent(model))
	}
	return events, total, nil
}

func (r *GormOutboxRepository) FindByID(ctx context.Context, id int64) (*OutboxEvent, error) {
	var model OutboxEventModel
	if err := r.db.WithContext(ctx).First(&model, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOutboxEventNotFound
		}
		return nil, fmt.Errorf("find outbox event: %w", err)
	}
	event := outboxModelToEvent(model)
	return &event, nil
}

func (r *GormOutboxRepository) MarkSent(ctx context.Context, id int64, sentAt time.Time) (*OutboxEvent, error) {
	return r.update(ctx, id, map[string]any{
		"status":     OutboxStatusSent,
		"sent_at":    sentAt,
		"updated_at": sentAt,
		"last_error": "",
	})
}

func (r *GormOutboxRepository) MarkFailed(ctx context.Context, id int64, attempts int, nextAttemptAt time.Time, lastError string) (*OutboxEvent, error) {
	return r.update(ctx, id, map[string]any{
		"status":          OutboxStatusFailed,
		"attempts":        attempts,
		"next_attempt_at": nextAttemptAt,
		"last_error":      lastError,
		"updated_at":      nextAttemptAt,
	})
}

func (r *GormOutboxRepository) update(ctx context.Context, id int64, updates map[string]any) (*OutboxEvent, error) {
	result := r.db.WithContext(ctx).
		Model(&OutboxEventModel{}).
		Where("id = ?", id).
		Updates(updates)
	if result.Error != nil {
		return nil, fmt.Errorf("update outbox event: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, ErrOutboxEventNotFound
	}
	return r.FindByID(ctx, id)
}

func outboxModelToEvent(model OutboxEventModel) OutboxEvent {
	return OutboxEvent{
		ID:            model.ID,
		Topic:         model.Topic,
		Key:           model.EventKey,
		Value:         []byte(model.PayloadJSON),
		Status:        model.Status,
		Attempts:      model.Attempts,
		NextAttemptAt: model.NextAttemptAt,
		LastError:     model.LastError,
		CreatedAt:     model.CreatedAt,
		UpdatedAt:     model.UpdatedAt,
		SentAt:        model.SentAt,
	}
}
