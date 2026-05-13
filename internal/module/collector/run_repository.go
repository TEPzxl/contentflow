package collector

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
)

var ErrCollectionRunNotFound = fmt.Errorf("collection run not found")

type RunRepository interface {
	Create(ctx context.Context, run *CollectionRun) error
	Finish(ctx context.Context, params FinishRunParams) error
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
