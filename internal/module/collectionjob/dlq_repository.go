package collectionjob

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var ErrDLQItemNotFound = errors.New("dlq item not found")

type DLQItemModel struct {
	ID             int64          `gorm:"column:id;primaryKey"`
	TaskID         string         `gorm:"column:task_id;type:varchar(100);not null;index"`
	UserID         int64          `gorm:"column:user_id;not null;index"`
	SourceID       int64          `gorm:"column:source_id;not null;index"`
	IdempotencyKey string         `gorm:"column:idempotency_key;type:varchar(200);not null;index"`
	Attempt        int            `gorm:"column:attempt;not null"`
	ErrorMessage   string         `gorm:"column:error_message;type:text;not null"`
	PayloadJSON    datatypes.JSON `gorm:"column:payload_json;type:jsonb;not null"`
	Status         string         `gorm:"column:status;type:varchar(50);not null;index"`
	CreatedAt      time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt      time.Time      `gorm:"column:updated_at;not null"`
	ReplayedAt     *time.Time     `gorm:"column:replayed_at"`
	HandledAt      *time.Time     `gorm:"column:handled_at"`
}

func (DLQItemModel) TableName() string {
	return "collection_dlq_items"
}

type GormDLQRepository struct {
	db *gorm.DB
}

func NewGormDLQRepository(db *gorm.DB) DLQRepository {
	return &GormDLQRepository{db: db}
}

func (r *GormDLQRepository) Create(ctx context.Context, params CreateDLQItemParams) (*DLQItem, error) {
	payload, err := json.Marshal(params.Event)
	if err != nil {
		return nil, fmt.Errorf("marshal dlq payload: %w", err)
	}
	now := params.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}

	model := DLQItemModel{
		TaskID:         params.Event.TaskID,
		UserID:         params.Event.UserID,
		SourceID:       params.Event.SourceID,
		IdempotencyKey: params.Event.IdempotencyKey,
		Attempt:        params.Event.Attempt,
		ErrorMessage:   params.ErrorMessage,
		PayloadJSON:    datatypes.JSON(payload),
		Status:         DLQStatusPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return nil, fmt.Errorf("create dlq item: %w", err)
	}

	return dlqModelToItem(model)
}

func (r *GormDLQRepository) List(ctx context.Context, params ListDLQItemsParams) ([]DLQItem, int64, error) {
	limit := normalizeLimit(params.Limit)
	offset := normalizeOffset(params.Offset)

	query := r.db.WithContext(ctx).Model(&DLQItemModel{})
	if params.UserID > 0 {
		query = query.Where("user_id = ?", params.UserID)
	}
	if params.Status != "" {
		query = query.Where("status = ?", params.Status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count dlq items: %w", err)
	}

	var models []DLQItemModel
	if err := query.
		Order("created_at DESC").
		Order("id DESC").
		Limit(limit).
		Offset(offset).
		Find(&models).Error; err != nil {
		return nil, 0, fmt.Errorf("list dlq items: %w", err)
	}

	items := make([]DLQItem, 0, len(models))
	for _, model := range models {
		item, err := dlqModelToItem(model)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *item)
	}
	return items, total, nil
}

func (r *GormDLQRepository) FindByUserIDAndID(ctx context.Context, userID, id int64) (*DLQItem, error) {
	var model DLQItemModel
	query := r.db.WithContext(ctx).Where("id = ?", id)
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	if err := query.First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDLQItemNotFound
		}
		return nil, fmt.Errorf("find dlq item: %w", err)
	}
	return dlqModelToItem(model)
}

func (r *GormDLQRepository) FindByID(ctx context.Context, id int64) (*DLQItem, error) {
	return r.FindByUserIDAndID(ctx, 0, id)
}

func (r *GormDLQRepository) MarkReplayed(ctx context.Context, id int64, replayedAt time.Time) (*DLQItem, error) {
	return r.updateStatus(ctx, id, DLQStatusReplayed, "replayed_at", replayedAt)
}

func (r *GormDLQRepository) MarkHandled(ctx context.Context, id int64, handledAt time.Time) (*DLQItem, error) {
	return r.updateStatus(ctx, id, DLQStatusHandled, "handled_at", handledAt)
}

func (r *GormDLQRepository) updateStatus(ctx context.Context, id int64, status string, timeColumn string, at time.Time) (*DLQItem, error) {
	updates := map[string]any{
		"status":     status,
		"updated_at": at,
		timeColumn:   at,
	}
	result := r.db.WithContext(ctx).
		Model(&DLQItemModel{}).
		Where("id = ?", id).
		Updates(updates)
	if result.Error != nil {
		return nil, fmt.Errorf("update dlq item: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, ErrDLQItemNotFound
	}
	return r.FindByID(ctx, id)
}

func dlqModelToItem(model DLQItemModel) (*DLQItem, error) {
	var payload CollectionRequested
	if err := json.Unmarshal(model.PayloadJSON, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal dlq payload: %w", err)
	}

	return &DLQItem{
		ID:             model.ID,
		TaskID:         model.TaskID,
		UserID:         model.UserID,
		SourceID:       model.SourceID,
		IdempotencyKey: model.IdempotencyKey,
		Attempt:        model.Attempt,
		ErrorMessage:   model.ErrorMessage,
		Payload:        payload,
		Status:         model.Status,
		CreatedAt:      model.CreatedAt,
		UpdatedAt:      model.UpdatedAt,
		ReplayedAt:     model.ReplayedAt,
		HandledAt:      model.HandledAt,
	}, nil
}
