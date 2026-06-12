package collectionjob

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	migratepostgres "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestGormOutboxRepository_ClaimReadySkipsActiveClaims(t *testing.T) {
	db, cleanup := setupOutboxRepositoryTestDB(t)
	defer cleanup()

	repo := NewGormOutboxRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC)
	lockedUntil := now.Add(time.Minute)

	first, err := repo.Create(ctx, CreateOutboxEventParams{
		Topic:     TopicCollectionRequested,
		Key:       "collection:source:1",
		Payload:   CollectionRequested{TaskID: "task-1", SourceID: 1},
		CreatedAt: now,
	})
	if err != nil {
		t.Fatalf("Create(first) error = %v", err)
	}
	second, err := repo.Create(ctx, CreateOutboxEventParams{
		Topic:     TopicCollectionRequested,
		Key:       "collection:source:2",
		Payload:   CollectionRequested{TaskID: "task-2", SourceID: 2},
		CreatedAt: now,
	})
	if err != nil {
		t.Fatalf("Create(second) error = %v", err)
	}

	claimedByFirst, total, err := repo.ClaimReady(ctx, now, 1, "claim-1", lockedUntil)
	if err != nil {
		t.Fatalf("ClaimReady(first) error = %v", err)
	}
	if total != 2 || len(claimedByFirst) != 1 || claimedByFirst[0].ID != first.ID {
		t.Fatalf("first claim total/events = %d/%#v, want first event only", total, claimedByFirst)
	}

	claimedBySecond, total, err := repo.ClaimReady(ctx, now, 10, "claim-2", lockedUntil)
	if err != nil {
		t.Fatalf("ClaimReady(second) error = %v", err)
	}
	if total != 1 || len(claimedBySecond) != 1 || claimedBySecond[0].ID != second.ID {
		t.Fatalf("second claim total/events = %d/%#v, want second event only", total, claimedBySecond)
	}

	firstAfterClaim, err := repo.FindByID(ctx, first.ID)
	if err != nil {
		t.Fatalf("FindByID(first) error = %v", err)
	}
	if firstAfterClaim.Status != OutboxStatusProcessing || firstAfterClaim.ClaimID != "claim-1" || !firstAfterClaim.LockedUntil.Equal(lockedUntil) {
		t.Fatalf("first after claim = %#v, want processing with claim-1 lease", firstAfterClaim)
	}
}

func TestGormOutboxRepository_ClaimReadyReclaimsExpiredProcessingEvents(t *testing.T) {
	db, cleanup := setupOutboxRepositoryTestDB(t)
	defer cleanup()

	repo := NewGormOutboxRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC)

	event, err := repo.Create(ctx, CreateOutboxEventParams{
		Topic:     TopicCollectionRequested,
		Key:       "collection:source:1",
		Payload:   CollectionRequested{TaskID: "task-1", SourceID: 1},
		CreatedAt: now,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, _, err := repo.ClaimReady(ctx, now, 1, "expired-claim", now.Add(-time.Minute)); err != nil {
		t.Fatalf("ClaimReady(expired) error = %v", err)
	}

	reclaimed, total, err := repo.ClaimReady(ctx, now, 1, "new-claim", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("ClaimReady(reclaim) error = %v", err)
	}
	if total != 1 || len(reclaimed) != 1 || reclaimed[0].ID != event.ID || reclaimed[0].ClaimID != "new-claim" {
		t.Fatalf("reclaimed total/events = %d/%#v, want event with new claim", total, reclaimed)
	}
}

func TestGormOutboxRepository_EnforcesOneActiveEventPerTopicKey(t *testing.T) {
	db, cleanup := setupOutboxRepositoryTestDB(t)
	defer cleanup()

	now := time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC)
	first := OutboxEventModel{
		Topic:         TopicCollectionRequested,
		EventKey:      "collection:source:42",
		PayloadJSON:   []byte(`{"task_id":"task-1","user_id":100,"source_id":42,"idempotency_key":"collection:source:42","requested_at":"2026-05-29T10:00:00Z"}`),
		Status:        OutboxStatusPending,
		Attempts:      0,
		NextAttemptAt: now,
		LastError:     "",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := db.Create(&first).Error; err != nil {
		t.Fatalf("Create(first) error = %v", err)
	}

	duplicate := first
	duplicate.ID = 0
	duplicate.PayloadJSON = []byte(`{"task_id":"task-2","user_id":100,"source_id":42,"idempotency_key":"collection:source:42","requested_at":"2026-05-29T10:00:00Z"}`)
	if err := db.Create(&duplicate).Error; err == nil {
		t.Fatal("Create(duplicate active) error is nil, want unique constraint violation")
	}

	if err := db.Model(&OutboxEventModel{}).Where("id = ?", first.ID).Updates(map[string]any{
		"status":     OutboxStatusSent,
		"sent_at":    now,
		"updated_at": now,
	}).Error; err != nil {
		t.Fatalf("mark first sent: %v", err)
	}
	duplicate.ID = 0
	duplicate.Status = OutboxStatusPending
	if err := db.Create(&duplicate).Error; err != nil {
		t.Fatalf("Create(after sent) error = %v", err)
	}
}

func TestGormOutboxRepository_MarkSentRequiresMatchingClaim(t *testing.T) {
	db, cleanup := setupOutboxRepositoryTestDB(t)
	defer cleanup()

	repo := NewGormOutboxRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC)

	event, err := repo.Create(ctx, CreateOutboxEventParams{
		Topic:     TopicCollectionRequested,
		Key:       "collection:source:1",
		Payload:   CollectionRequested{TaskID: "task-1", SourceID: 1},
		CreatedAt: now,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, _, err := repo.ClaimReady(ctx, now, 1, "claim-1", now.Add(time.Minute)); err != nil {
		t.Fatalf("ClaimReady() error = %v", err)
	}

	if _, err := repo.MarkSent(ctx, event.ID, "wrong-claim", now); !errors.Is(err, ErrOutboxClaimLost) {
		t.Fatalf("MarkSent(wrong claim) error = %v, want %v", err, ErrOutboxClaimLost)
	}
	item, err := repo.FindByID(ctx, event.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if item.Status != OutboxStatusProcessing {
		t.Fatalf("status after wrong claim = %s, want %s", item.Status, OutboxStatusProcessing)
	}

	if _, err := repo.MarkSent(ctx, event.ID, "claim-1", now); err != nil {
		t.Fatalf("MarkSent(matching claim) error = %v", err)
	}
	item, err = repo.FindByID(ctx, event.ID)
	if err != nil {
		t.Fatalf("FindByID(sent) error = %v", err)
	}
	if item.Status != OutboxStatusSent || item.SentAt == nil || item.ClaimID != "" || !item.LockedUntil.IsZero() {
		t.Fatalf("sent item = %#v, want sent with cleared claim", item)
	}
}

func setupOutboxRepositoryTestDB(t *testing.T) (*gorm.DB, func()) {
	t.Helper()

	ctx := context.Background()

	pgContainer, err := tcpostgres.Run(
		ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("contentflow_test"),
		tcpostgres.WithUsername("contentflow"),
		tcpostgres.WithPassword("contentflow"),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("5432/tcp").WithStartupTimeout(30*time.Second)),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}

	cleanup := func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("terminate postgres container: %v", err)
		}
	}

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		cleanup()
		t.Fatalf("get postgres connection string: %v", err)
	}

	sqlDB, err := sql.Open("postgres", connStr)
	if err != nil {
		cleanup()
		t.Fatalf("open sql db: %v", err)
	}

	if err := runOutboxMigrations(t, sqlDB); err != nil {
		_ = sqlDB.Close()
		cleanup()
		t.Fatalf("run migrations: %v", err)
	}

	db, err := gorm.Open(gormpostgres.Open(connStr), &gorm.Config{TranslateError: true})
	if err != nil {
		_ = sqlDB.Close()
		cleanup()
		t.Fatalf("open gorm db: %v", err)
	}

	return db, func() {
		gormSQLDB, err := db.DB()
		if err == nil {
			_ = gormSQLDB.Close()
		}
		_ = sqlDB.Close()
		cleanup()
	}
}

func runOutboxMigrations(t *testing.T, db *sql.DB) error {
	t.Helper()

	driver, err := migratepostgres.WithInstance(db, &migratepostgres.Config{})
	if err != nil {
		return fmt.Errorf("create migrate postgres driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://"+outboxMigrationsDir(t),
		"postgres",
		driver,
	)
	if err != nil {
		return fmt.Errorf("create migrate instance: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}

	return nil
}

func outboxMigrationsDir(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("get current file path")
	}

	projectRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	return filepath.Join(projectRoot, "migrations")
}
