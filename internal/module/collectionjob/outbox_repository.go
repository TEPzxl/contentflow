package collectionjob

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrOutboxEventNotFound = errors.New("outbox event not found")
	ErrOutboxClaimLost     = errors.New("outbox event claim lost")
)

type OutboxEventModel struct {
	ID            int64          `gorm:"column:id;primaryKey"`
	Topic         string         `gorm:"column:topic;type:varchar(200);not null;index"`
	EventKey      string         `gorm:"column:event_key;type:varchar(300);not null;index"`
	PayloadJSON   datatypes.JSON `gorm:"column:payload_json;type:jsonb;not null"`
	Status        string         `gorm:"column:status;type:varchar(50);not null;index"`
	Attempts      int            `gorm:"column:attempts;not null"`
	NextAttemptAt time.Time      `gorm:"column:next_attempt_at;not null;index"`
	LastError     string         `gorm:"column:last_error;type:text;not null"`
	ClaimID       string         `gorm:"column:claim_id;type:varchar(100);not null;index"`
	LockedUntil   *time.Time     `gorm:"column:locked_until;index"`
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

func (r *GormOutboxRepository) ClaimReady(ctx context.Context, now time.Time, limit int, claimID string, lockedUntil time.Time) ([]OutboxEvent, int64, error) {
	if limit <= 0 {
		limit = 100
	}
	if claimID == "" {
		return nil, 0, fmt.Errorf("claim ready outbox events: empty claim id")
	}

	readyFilter, readyArgs := outboxReadyForClaimFilter(now)
	var total int64
	var claimed []OutboxEventModel

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&OutboxEventModel{}).Where(readyFilter, readyArgs...).Count(&total).Error; err != nil {
			return fmt.Errorf("count claimable outbox events: %w", err)
		}

		var models []OutboxEventModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where(readyFilter, readyArgs...).
			Order("next_attempt_at ASC").
			Order("id ASC").
			Limit(limit).
			Find(&models).Error; err != nil {
			return fmt.Errorf("claim ready outbox events: %w", err)
		}
		if len(models) == 0 {
			return nil
		}

		ids := make([]int64, 0, len(models))
		for _, model := range models {
			ids = append(ids, model.ID)
		}

		if err := tx.Model(&OutboxEventModel{}).
			Where("id IN ?", ids).
			Updates(map[string]any{
				"status":       OutboxStatusProcessing,
				"claim_id":     claimID,
				"locked_until": lockedUntil,
				"updated_at":   now,
			}).Error; err != nil {
			return fmt.Errorf("mark outbox events processing: %w", err)
		}

		return tx.Where("id IN ?", ids).
			Order("next_attempt_at ASC").
			Order("id ASC").
			Find(&claimed).Error
	})
	if err != nil {
		return nil, 0, err
	}

	events := make([]OutboxEvent, 0, len(claimed))
	for _, model := range claimed {
		events = append(events, outboxModelToEvent(model))
	}
	return events, total, nil
}

func outboxReadyForClaimFilter(now time.Time) (string, []any) {
	return "(status IN ? AND next_attempt_at <= ?) OR (status = ? AND locked_until <= ?)", []any{
		[]string{OutboxStatusPending, OutboxStatusFailed},
		now,
		OutboxStatusProcessing,
		now,
	}
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

func (r *GormOutboxRepository) MarkSent(ctx context.Context, id int64, claimID string, sentAt time.Time) (*OutboxEvent, error) {
	return r.updateClaimed(ctx, id, claimID, map[string]any{
		"status":       OutboxStatusSent,
		"sent_at":      sentAt,
		"updated_at":   sentAt,
		"last_error":   "",
		"claim_id":     "",
		"locked_until": nil,
	})
}

func (r *GormOutboxRepository) MarkFailed(ctx context.Context, id int64, claimID string, attempts int, nextAttemptAt time.Time, lastError string) (*OutboxEvent, error) {
	return r.updateClaimed(ctx, id, claimID, map[string]any{
		"status":          OutboxStatusFailed,
		"attempts":        attempts,
		"next_attempt_at": nextAttemptAt,
		"last_error":      lastError,
		"updated_at":      nextAttemptAt,
		"claim_id":        "",
		"locked_until":    nil,
	})
}

func (r *GormOutboxRepository) updateClaimed(ctx context.Context, id int64, claimID string, updates map[string]any) (*OutboxEvent, error) {
	result := r.db.WithContext(ctx).
		Model(&OutboxEventModel{}).
		Where("id = ? AND status = ? AND claim_id = ?", id, OutboxStatusProcessing, claimID).
		Updates(updates)
	if result.Error != nil {
		return nil, fmt.Errorf("update outbox event: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, ErrOutboxClaimLost
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
		ClaimID:       model.ClaimID,
		LockedUntil:   outboxLockedUntil(model.LockedUntil),
		CreatedAt:     model.CreatedAt,
		UpdatedAt:     model.UpdatedAt,
		SentAt:        model.SentAt,
	}
}

func outboxLockedUntil(lockedUntil *time.Time) time.Time {
	if lockedUntil == nil {
		return time.Time{}
	}
	return *lockedUntil
}
