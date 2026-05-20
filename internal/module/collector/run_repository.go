package collector

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

var ErrCollectionRunNotFound = fmt.Errorf("collection run not found")

type RunRepository interface {
	Create(ctx context.Context, run *CollectionRun) error
	Finish(ctx context.Context, params FinishRunParams) error
	ListBySourceID(ctx context.Context, params ListRunsParams) ([]CollectionRun, int64, error)
	FindByUserIDAndID(ctx context.Context, userID, runID int64) (*CollectionRun, error)
}

type ListRunsParams struct {
	SourceID int64
	Status   string
	Limit    int
	Offset   int
}

type FinishRunParams struct {
	RunID           int64
	Status          string
	FinishedAt      time.Time
	FetchedCount    int
	InsertedCount   int
	DuplicatedCount int
	ErrorMessage    string
}

type GormRunRepository struct {
	db *gorm.DB
}

func NewRunRepository(db *gorm.DB) RunRepository {
	return &GormRunRepository{db: db}
}

func (r *GormRunRepository) Create(ctx context.Context, run *CollectionRun) error {
	if err := gorm.G[CollectionRun](r.db).Create(ctx, run); err != nil {
		return fmt.Errorf("create collection run: %w", err)
	}
	return nil
}

func (r *GormRunRepository) Finish(ctx context.Context, params FinishRunParams) error {
	rowAffected, err := gorm.G[CollectionRun](r.db).
		Where("id = ?", params.RunID).
		Select("status", "finished_at", "fetched_count", "inserted_count", "duplicated_count", "error_message").
		Updates(ctx, CollectionRun{
			Status:          params.Status,
			FinishedAt:      &params.FinishedAt,
			FetchedCount:    params.FetchedCount,
			InsertedCount:   params.InsertedCount,
			DuplicatedCount: params.DuplicatedCount,
			ErrorMessage:    params.ErrorMessage,
		})
	if err != nil {
		return fmt.Errorf("finish collection run: %w", err)
	}

	if rowAffected == 0 {
		return ErrCollectionRunNotFound
	}
	return nil
}

func (r *GormRunRepository) ListBySourceID(ctx context.Context, params ListRunsParams) ([]CollectionRun, int64, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	offset := params.Offset
	if offset < 0 {
		offset = 0
	}

	query := r.db.WithContext(ctx).
		Model(&CollectionRun{}).
		Where("source_id = ?", params.SourceID)
	if params.Status != "" {
		query = query.Where("status = ?", params.Status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count collection runs: %w", err)
	}

	var runs []CollectionRun
	if err := query.
		Order("started_at DESC").
		Order("id DESC").
		Limit(limit).
		Offset(offset).
		Find(&runs).Error; err != nil {
		return nil, 0, fmt.Errorf("list collection runs: %w", err)
	}
	return runs, total, nil
}

func (r *GormRunRepository) FindByUserIDAndID(ctx context.Context, userID, runID int64) (*CollectionRun, error) {
	var run CollectionRun
	if err := r.db.WithContext(ctx).
		Model(&CollectionRun{}).
		Joins("JOIN sources ON sources.id = collection_runs.source_id").
		Where("collection_runs.id = ?", runID).
		Where("sources.user_id = ?", userID).
		Where("sources.deleted_at IS NULL").
		First(&run).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCollectionRunNotFound
		}
		return nil, fmt.Errorf("find collection run: %w", err)
	}
	return &run, nil
}
