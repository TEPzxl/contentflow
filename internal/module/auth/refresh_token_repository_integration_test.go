package auth_test

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
	"github.com/tepzxl/contentflow/internal/module/auth"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestRefreshTokenRepository_RevokeByHashConsumesOnlyValidToken(t *testing.T) {
	db, cleanup := setupAuthRepositoryTestDB(t)
	defer cleanup()

	ctx := context.Background()
	repo := auth.NewRefreshTokenRepository(db)
	userID := createAuthRepositoryTestUser(t, db, "refresh-consume@example.com")
	now := time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC)

	if err := repo.Create(ctx, &auth.RefreshToken{
		UserID:    userID,
		TokenHash: "refresh-token-hash",
		ExpiresAt: now.Add(time.Hour),
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := repo.RevokeByHash(ctx, "refresh-token-hash", now.Add(time.Minute)); err != nil {
		t.Fatalf("first RevokeByHash() error = %v", err)
	}

	if err := repo.RevokeByHash(ctx, "refresh-token-hash", now.Add(2*time.Minute)); !errors.Is(err, auth.ErrRefreshTokenNotFound) {
		t.Fatalf("second RevokeByHash() error = %v, want ErrRefreshTokenNotFound", err)
	}
}

func setupAuthRepositoryTestDB(t *testing.T) (*gorm.DB, func()) {
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

	if err := runAuthRepositoryMigrations(t, sqlDB); err != nil {
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

func createAuthRepositoryTestUser(t *testing.T, db *gorm.DB, email string) int64 {
	t.Helper()

	now := time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC)
	var id int64
	if err := db.Raw(
		`INSERT INTO users (email, password_hash, display_name, created_at, updated_at) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		email,
		"password-hash",
		"Auth Test",
		now,
		now,
	).Scan(&id).Error; err != nil {
		t.Fatalf("create auth repository test user: %v", err)
	}
	return id
}

func runAuthRepositoryMigrations(t *testing.T, db *sql.DB) error {
	t.Helper()

	driver, err := migratepostgres.WithInstance(db, &migratepostgres.Config{})
	if err != nil {
		return fmt.Errorf("create migrate postgres driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://"+authRepositoryMigrationsDir(t),
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

func authRepositoryMigrationsDir(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("get current file path")
	}
	projectRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	return filepath.Join(projectRoot, "migrations")
}
