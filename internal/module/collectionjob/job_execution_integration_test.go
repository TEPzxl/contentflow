package collectionjob

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestGormJobExecutionRepository_ClaimSkipsCompletedTask(t *testing.T) {
	db, cleanup := setupOutboxRepositoryTestDB(t)
	defer cleanup()

	sourceID := createJobExecutionSource(t, db)
	repo := NewGormJobExecutionRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	event := CollectionRequested{
		TaskID:         "task-completed",
		IdempotencyKey: "collection:source:42",
		SourceID:       sourceID,
		Attempt:        0,
		RequestedAt:    now,
	}

	claimed, shouldProcess, err := repo.Claim(ctx, event, "claim-1", now, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("Claim(first) error = %v", err)
	}
	if !shouldProcess || claimed.Status != JobExecutionStatusProcessing {
		t.Fatalf("first claim = %#v shouldProcess=%v, want processing", claimed, shouldProcess)
	}
	if _, err := repo.MarkSucceeded(ctx, event.TaskID, "claim-1", 0, now); err != nil {
		t.Fatalf("MarkSucceeded() error = %v", err)
	}

	claimed, shouldProcess, err = repo.Claim(ctx, event, "claim-2", now.Add(time.Second), now.Add(time.Minute))
	if err != nil {
		t.Fatalf("Claim(duplicate) error = %v", err)
	}
	if shouldProcess || claimed.Status != JobExecutionStatusSucceeded {
		t.Fatalf("duplicate claim = %#v shouldProcess=%v, want succeeded skip", claimed, shouldProcess)
	}
}

func TestGormJobExecutionRepository_ClaimAllowsOnlyNewerFailedAttempt(t *testing.T) {
	db, cleanup := setupOutboxRepositoryTestDB(t)
	defer cleanup()

	sourceID := createJobExecutionSource(t, db)
	repo := NewGormJobExecutionRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	event := CollectionRequested{
		TaskID:         "task-retry",
		IdempotencyKey: "collection:source:42",
		SourceID:       sourceID,
		Attempt:        0,
		RequestedAt:    now,
	}

	if _, shouldProcess, err := repo.Claim(ctx, event, "claim-1", now, now.Add(time.Minute)); err != nil || !shouldProcess {
		t.Fatalf("Claim(first) shouldProcess=%v error=%v, want true nil", shouldProcess, err)
	}
	if _, err := repo.MarkFailed(ctx, event.TaskID, "claim-1", event.Attempt, "temporary", now); err != nil {
		t.Fatalf("MarkFailed() error = %v", err)
	}

	stale, shouldProcess, err := repo.Claim(ctx, event, "claim-stale", now.Add(time.Second), now.Add(time.Minute))
	if err != nil {
		t.Fatalf("Claim(stale) error = %v", err)
	}
	if shouldProcess || stale.Status != JobExecutionStatusFailed || stale.Attempt != 0 {
		t.Fatalf("stale claim = %#v shouldProcess=%v, want failed skip", stale, shouldProcess)
	}

	event.Attempt = 1
	retry, shouldProcess, err := repo.Claim(ctx, event, "claim-retry", now.Add(2*time.Second), now.Add(time.Minute))
	if err != nil {
		t.Fatalf("Claim(retry) error = %v", err)
	}
	if !shouldProcess || retry.Status != JobExecutionStatusProcessing || retry.Attempt != 1 || retry.ClaimID != "claim-retry" {
		t.Fatalf("retry claim = %#v shouldProcess=%v, want processing attempt 1", retry, shouldProcess)
	}
}

func TestGormJobExecutionRepository_MarkSucceededRequiresMatchingClaim(t *testing.T) {
	db, cleanup := setupOutboxRepositoryTestDB(t)
	defer cleanup()

	sourceID := createJobExecutionSource(t, db)
	repo := NewGormJobExecutionRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	event := CollectionRequested{
		TaskID:         "task-fenced",
		IdempotencyKey: "collection:source:42",
		SourceID:       sourceID,
		Attempt:        0,
		RequestedAt:    now,
	}

	if _, shouldProcess, err := repo.Claim(ctx, event, "claim-1", now, now.Add(time.Minute)); err != nil || !shouldProcess {
		t.Fatalf("Claim() shouldProcess=%v error=%v, want true nil", shouldProcess, err)
	}
	if _, err := repo.MarkSucceeded(ctx, event.TaskID, "wrong-claim", 99, now); !errors.Is(err, ErrJobExecutionClaimLost) {
		t.Fatalf("MarkSucceeded(wrong claim) error = %v, want %v", err, ErrJobExecutionClaimLost)
	}
}

func createJobExecutionSource(t *testing.T, db *gorm.DB) int64 {
	t.Helper()

	var userID int64
	if err := db.Raw(`
		INSERT INTO users (email, password_hash, display_name, created_at, updated_at)
		VALUES (?, 'hashed-password', 'test user', NOW(), NOW())
		RETURNING id
	`, "job-execution@example.com").Scan(&userID).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	var sourceID int64
	if err := db.Raw(`
		INSERT INTO sources (user_id, name, type, url, config_json, is_active, last_fetch_status, last_fetch_message, created_at, updated_at)
		VALUES (?, 'Go Blog', 'rss', 'https://example.com/feed.xml', '{}'::jsonb, true, '', '', NOW(), NOW())
		RETURNING id
	`, userID).Scan(&sourceID).Error; err != nil {
		t.Fatalf("create source: %v", err)
	}

	return sourceID
}
