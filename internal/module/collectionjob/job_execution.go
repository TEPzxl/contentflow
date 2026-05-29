package collectionjob

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	JobExecutionStatusProcessing = "processing"
	JobExecutionStatusSucceeded  = "succeeded"
	JobExecutionStatusFailed     = "failed"
	JobExecutionStatusDLQ        = "dlq"
)

var ErrJobExecutionClaimLost = errors.New("job execution claim lost")

type JobExecution struct {
	ID             int64
	TaskID         string
	IdempotencyKey string
	SourceID       int64
	Status         string
	Attempt        int
	RunID          int64
	LastError      string
	ClaimID        string
	LockedUntil    time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type JobExecutionRepository interface {
	Claim(ctx context.Context, event CollectionRequested, claimID string, now time.Time, lockedUntil time.Time) (*JobExecution, bool, error)
	MarkSucceeded(ctx context.Context, taskID string, claimID string, runID int64, now time.Time) (*JobExecution, error)
	MarkFailed(ctx context.Context, taskID string, claimID string, attempt int, errMessage string, now time.Time) (*JobExecution, error)
	MarkDLQ(ctx context.Context, taskID string, claimID string, errMessage string, now time.Time) (*JobExecution, error)
}

type JobExecutionModel struct {
	ID             int64      `gorm:"column:id;primaryKey"`
	TaskID         string     `gorm:"column:task_id;type:varchar(100);not null;uniqueIndex"`
	IdempotencyKey string     `gorm:"column:idempotency_key;type:varchar(200);not null;index"`
	SourceID       int64      `gorm:"column:source_id;not null;index"`
	Status         string     `gorm:"column:status;type:varchar(50);not null;index"`
	Attempt        int        `gorm:"column:attempt;not null"`
	RunID          *int64     `gorm:"column:run_id"`
	LastError      string     `gorm:"column:last_error;type:text;not null"`
	ClaimID        string     `gorm:"column:claim_id;type:varchar(100);not null;index"`
	LockedUntil    *time.Time `gorm:"column:locked_until;index"`
	CreatedAt      time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt      time.Time  `gorm:"column:updated_at;not null"`
}

func (JobExecutionModel) TableName() string {
	return "collection_job_executions"
}

type GormJobExecutionRepository struct {
	db *gorm.DB
}

func NewGormJobExecutionRepository(db *gorm.DB) JobExecutionRepository {
	return &GormJobExecutionRepository{db: db}
}

func (r *GormJobExecutionRepository) Claim(ctx context.Context, event CollectionRequested, claimID string, now time.Time, lockedUntil time.Time) (*JobExecution, bool, error) {
	if event.TaskID == "" {
		return nil, false, fmt.Errorf("claim job execution: empty task id")
	}
	if claimID == "" {
		return nil, false, fmt.Errorf("claim job execution: empty claim id")
	}

	var claimed JobExecutionModel
	shouldProcess := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		model := JobExecutionModel{
			TaskID:         event.TaskID,
			IdempotencyKey: event.IdempotencyKey,
			SourceID:       event.SourceID,
			Status:         JobExecutionStatusProcessing,
			Attempt:        event.Attempt,
			LastError:      "",
			ClaimID:        claimID,
			LockedUntil:    &lockedUntil,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model)
		if result.Error != nil {
			return fmt.Errorf("create job execution claim: %w", result.Error)
		}
		if result.RowsAffected == 1 {
			claimed = model
			shouldProcess = true
			return nil
		}

		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&claimed, "task_id = ?", event.TaskID).Error; err != nil {
			return fmt.Errorf("find job execution claim: %w", err)
		}
		if !jobExecutionShouldProcess(claimed, event, now) {
			shouldProcess = false
			return nil
		}

		updates := map[string]any{
			"idempotency_key": event.IdempotencyKey,
			"source_id":       event.SourceID,
			"status":          JobExecutionStatusProcessing,
			"attempt":         event.Attempt,
			"last_error":      "",
			"claim_id":        claimID,
			"locked_until":    lockedUntil,
			"updated_at":      now,
		}
		if err := tx.Model(&JobExecutionModel{}).Where("task_id = ?", event.TaskID).Updates(updates).Error; err != nil {
			return fmt.Errorf("update job execution claim: %w", err)
		}
		shouldProcess = true
		return tx.First(&claimed, "task_id = ?", event.TaskID).Error
	})
	if err != nil {
		return nil, false, err
	}
	execution := jobExecutionModelToExecution(claimed)
	return &execution, shouldProcess, nil
}

func jobExecutionShouldProcess(model JobExecutionModel, event CollectionRequested, now time.Time) bool {
	switch model.Status {
	case JobExecutionStatusSucceeded, JobExecutionStatusDLQ:
		return false
	case JobExecutionStatusProcessing:
		return event.Attempt > model.Attempt || model.LockedUntil == nil || !model.LockedUntil.After(now)
	case JobExecutionStatusFailed:
		return event.Attempt > model.Attempt
	default:
		return true
	}
}

func (r *GormJobExecutionRepository) MarkSucceeded(ctx context.Context, taskID string, claimID string, runID int64, now time.Time) (*JobExecution, error) {
	updates := map[string]any{
		"status":       JobExecutionStatusSucceeded,
		"run_id":       nil,
		"last_error":   "",
		"claim_id":     "",
		"locked_until": nil,
		"updated_at":   now,
	}
	if runID > 0 {
		updates["run_id"] = runID
	}
	return r.updateClaimed(ctx, taskID, claimID, updates)
}

func (r *GormJobExecutionRepository) MarkFailed(ctx context.Context, taskID string, claimID string, attempt int, errMessage string, now time.Time) (*JobExecution, error) {
	return r.updateClaimed(ctx, taskID, claimID, map[string]any{
		"status":       JobExecutionStatusFailed,
		"attempt":      attempt,
		"last_error":   errMessage,
		"claim_id":     "",
		"locked_until": nil,
		"updated_at":   now,
	})
}

func (r *GormJobExecutionRepository) MarkDLQ(ctx context.Context, taskID string, claimID string, errMessage string, now time.Time) (*JobExecution, error) {
	return r.updateClaimed(ctx, taskID, claimID, map[string]any{
		"status":       JobExecutionStatusDLQ,
		"last_error":   errMessage,
		"claim_id":     "",
		"locked_until": nil,
		"updated_at":   now,
	})
}

func (r *GormJobExecutionRepository) updateClaimed(ctx context.Context, taskID string, claimID string, updates map[string]any) (*JobExecution, error) {
	result := r.db.WithContext(ctx).
		Model(&JobExecutionModel{}).
		Where("task_id = ? AND status = ? AND claim_id = ?", taskID, JobExecutionStatusProcessing, claimID).
		Updates(updates)
	if result.Error != nil {
		return nil, fmt.Errorf("update job execution: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, ErrJobExecutionClaimLost
	}

	var model JobExecutionModel
	if err := r.db.WithContext(ctx).First(&model, "task_id = ?", taskID).Error; err != nil {
		return nil, fmt.Errorf("find job execution: %w", err)
	}
	execution := jobExecutionModelToExecution(model)
	return &execution, nil
}

func jobExecutionModelToExecution(model JobExecutionModel) JobExecution {
	return JobExecution{
		ID:             model.ID,
		TaskID:         model.TaskID,
		IdempotencyKey: model.IdempotencyKey,
		SourceID:       model.SourceID,
		Status:         model.Status,
		Attempt:        model.Attempt,
		RunID:          jobExecutionRunID(model.RunID),
		LastError:      model.LastError,
		ClaimID:        model.ClaimID,
		LockedUntil:    outboxLockedUntil(model.LockedUntil),
		CreatedAt:      model.CreatedAt,
		UpdatedAt:      model.UpdatedAt,
	}
}

func jobExecutionRunID(runID *int64) int64 {
	if runID == nil {
		return 0
	}
	return *runID
}
