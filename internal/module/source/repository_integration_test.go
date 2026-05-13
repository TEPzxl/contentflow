//go:build integration

package source_test

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
	"github.com/tepzxl/contentflow/internal/module/source"
	"github.com/tepzxl/contentflow/internal/module/user"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/datatypes"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	_ "github.com/lib/pq"
)

func TestSourceRepository_CreateAndFindByUserIDAndID(t *testing.T) {
	db, cleanup := setupSourceRepositoryTestDB(t)
	defer cleanup()

	repo := source.NewRepository(db)
	ctx := context.Background()

	rawURL := "https://go.dev/blog/feed.atom"

	src := &source.Source{
		UserID:           100,
		Name:             "Go Blog",
		Type:             source.TypeRSS,
		URL:              &rawURL,
		ConfigJSON:       datatypes.JSON([]byte(`{}`)),
		IsActive:         true,
		LastFetchStatus:  "",
		LastFetchMessage: "",
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}

	if err := repo.Create(ctx, src); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if src.ID == 0 {
		t.Fatal("Create() did not set source id")
	}

	got, err := repo.FindByUserIDAndID(ctx, 100, src.ID)
	if err != nil {
		t.Fatalf("FindByUserIDAndID() error = %v", err)
	}

	if got.ID != src.ID {
		t.Fatalf("ID = %d, want %d", got.ID, src.ID)
	}

	if got.UserID != 100 {
		t.Fatalf("UserID = %d, want 100", got.UserID)
	}

	if got.Name != "Go Blog" {
		t.Fatalf("Name = %s, want Go Blog", got.Name)
	}
}

func TestSourceRepository_FindByUserIDAndID_UserIsolation(t *testing.T) {
	db, cleanup := setupSourceRepositoryTestDB(t)
	defer cleanup()

	repo := source.NewRepository(db)
	ctx := context.Background()

	src := createTestSource(
		t,
		repo,
		100,
		"User 100 Source",
		source.TypeRSS,
		stringPtr("https://example.com/feed.xml"),
	)

	_, err := repo.FindByUserIDAndID(ctx, 200, src.ID)
	if !errors.Is(err, source.ErrSourceNotFound) {
		t.Fatalf("FindByUserIDAndID() error = %v, want ErrSourceNotFound", err)
	}
}
func TestSourceRepository_ListByUserID(t *testing.T) {
	db, cleanup := setupSourceRepositoryTestDB(t)
	defer cleanup()

	repo := source.NewRepository(db)
	ctx := context.Background()

	createTestSource(t, repo, 100, "RSS 1", source.TypeRSS, stringPtr("https://example.com/rss1.xml"))
	createTestSource(t, repo, 100, "RSS 2", source.TypeRSS, stringPtr("https://example.com/rss2.xml"))
	createTestSource(t, repo, 100, "Email 1", source.TypeEmail, nil)
	createTestSource(t, repo, 200, "Other User RSS", source.TypeRSS, stringPtr("https://other.example.com/rss.xml"))

	tests := []struct {
		name      string
		params    source.ListParams
		wantTotal int64
		wantLen   int
	}{
		{
			name: "list all user sources",
			params: source.ListParams{
				UserID: 100,
				Limit:  20,
				Offset: 0,
			},
			wantTotal: 3,
			wantLen:   3,
		},
		{
			name: "filter by rss",
			params: source.ListParams{
				UserID: 100,
				Type:   source.TypeRSS,
				Limit:  20,
				Offset: 0,
			},
			wantTotal: 2,
			wantLen:   2,
		},
		{
			name: "pagination",
			params: source.ListParams{
				UserID: 100,
				Limit:  1,
				Offset: 1,
			},
			wantTotal: 3,
			wantLen:   1,
		},
		{
			name: "other user isolation",
			params: source.ListParams{
				UserID: 200,
				Limit:  20,
				Offset: 0,
			},
			wantTotal: 1,
			wantLen:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, total, err := repo.ListByUserID(ctx, tt.params)
			if err != nil {
				t.Fatalf("ListByUserID() error = %v", err)
			}

			if total != tt.wantTotal {
				t.Fatalf("total = %d, want %d", total, tt.wantTotal)
			}

			if len(got) != tt.wantLen {
				t.Fatalf("len = %d, want %d", len(got), tt.wantLen)
			}
		})
	}
}
func TestSourceRepository_SoftDelete(t *testing.T) {
	db, cleanup := setupSourceRepositoryTestDB(t)
	defer cleanup()

	repo := source.NewRepository(db)
	ctx := context.Background()

	src := createTestSource(
		t,
		repo,
		100,
		"Go Blog",
		source.TypeRSS,
		stringPtr("https://example.com/feed.xml"),
	)

	deletedAt := time.Now().UTC()

	if err := repo.SoftDelete(ctx, 100, src.ID, deletedAt); err != nil {
		t.Fatalf("SoftDelete() error = %v", err)
	}

	_, err := repo.FindByUserIDAndID(ctx, 100, src.ID)
	if !errors.Is(err, source.ErrSourceNotFound) {
		t.Fatalf("FindByUserIDAndID() error = %v, want ErrSourceNotFound", err)
	}

	list, total, err := repo.ListByUserID(ctx, source.ListParams{
		UserID: 100,
		Limit:  20,
		Offset: 0,
	})
	if err != nil {
		t.Fatalf("ListByUserID() error = %v", err)
	}

	if total != 0 {
		t.Fatalf("total = %d, want 0", total)
	}

	if len(list) != 0 {
		t.Fatalf("len = %d, want 0", len(list))
	}

	var raw source.Source
	if err := db.
		Where("id = ?", src.ID).
		First(&raw).
		Error; err != nil {
		t.Fatalf("raw find soft deleted source: %v", err)
	}

	if raw.DeletedAt == nil {
		t.Fatal("DeletedAt is nil, want non-nil")
	}

	if raw.IsActive {
		t.Fatal("IsActive = true, want false")
	}
}
func TestSourceRepository_Create_DuplicatedURLSameUser(t *testing.T) {
	db, cleanup := setupSourceRepositoryTestDB(t)
	defer cleanup()

	repo := source.NewRepository(db)
	ctx := context.Background()

	rawURL := "https://example.com/feed.xml"

	createTestSource(t, repo, 100, "Source 1", source.TypeRSS, &rawURL)

	dup := &source.Source{
		UserID:           100,
		Name:             "Source 2",
		Type:             source.TypeRSS,
		URL:              &rawURL,
		ConfigJSON:       datatypes.JSON([]byte(`{}`)),
		IsActive:         true,
		LastFetchStatus:  "",
		LastFetchMessage: "",
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}

	err := repo.Create(ctx, dup)
	if !errors.Is(err, source.ErrSourceURLDuplicated) {
		t.Fatalf("Create() error = %v, want ErrSourceURLDuplicated", err)
	}
}
func TestSourceRepository_Create_SameURLDifferentUsers(t *testing.T) {
	db, cleanup := setupSourceRepositoryTestDB(t)
	defer cleanup()

	repo := source.NewRepository(db)

	rawURL := "https://example.com/feed.xml"

	createTestSource(t, repo, 100, "User 100 Source", source.TypeRSS, &rawURL)
	createTestSource(t, repo, 200, "User 200 Source", source.TypeRSS, &rawURL)
}

func TestSourceRepository_ListActiveForCollection(t *testing.T) {
	db, cleanup := setupSourceRepositoryTestDB(t)
	defer cleanup()

	repo := source.NewRepository(db)
	ctx := context.Background()

	activeRSS := createTestSource(t, repo, 100, "Active RSS", source.TypeRSS, stringPtr("https://example.com/active.xml"))
	activeEmail := createTestSource(t, repo, 200, "Active Email", source.TypeEmail, nil)
	inactive := createTestSource(t, repo, 100, "Inactive RSS", source.TypeRSS, stringPtr("https://example.com/inactive.xml"))
	deleted := createTestSource(t, repo, 100, "Deleted RSS", source.TypeRSS, stringPtr("https://example.com/deleted.xml"))

	inactive.IsActive = false
	inactive.UpdatedAt = time.Now().UTC()
	if err := repo.Update(ctx, inactive); err != nil {
		t.Fatalf("mark inactive source: %v", err)
	}

	if err := repo.SoftDelete(ctx, deleted.UserID, deleted.ID, time.Now().UTC()); err != nil {
		t.Fatalf("soft delete source: %v", err)
	}

	got, err := repo.ListActiveForCollection(ctx, 10)
	if err != nil {
		t.Fatalf("ListActiveForCollection() error = %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("len = %d, want 2: %#v", len(got), got)
	}

	gotByID := map[int64]source.ActiveSourceForCollection{}
	for _, src := range got {
		gotByID[src.ID] = src
	}

	if gotByID[activeRSS.ID].UserID != activeRSS.UserID {
		t.Fatalf("active rss user id = %d, want %d", gotByID[activeRSS.ID].UserID, activeRSS.UserID)
	}

	if gotByID[activeEmail.ID].UserID != activeEmail.UserID {
		t.Fatalf("active email user id = %d, want %d", gotByID[activeEmail.ID].UserID, activeEmail.UserID)
	}

	if _, ok := gotByID[inactive.ID]; ok {
		t.Fatalf("inactive source id %d returned", inactive.ID)
	}

	if _, ok := gotByID[deleted.ID]; ok {
		t.Fatalf("deleted source id %d returned", deleted.ID)
	}
}

func createTestSource(t *testing.T, repo source.Repository, userID int64, name string, sourceType string, rawURL *string) *source.Source {
	t.Helper()

	ctx := context.Background()
	now := time.Now().UTC()

	src := &source.Source{
		UserID:           userID,
		Name:             name,
		Type:             sourceType,
		URL:              rawURL,
		ConfigJSON:       datatypes.JSON([]byte(`{}`)),
		IsActive:         true,
		LastFetchStatus:  "",
		LastFetchMessage: "",
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := repo.Create(ctx, src); err != nil {
		t.Fatalf("create test source: %v", err)
	}

	if src.ID == 0 {
		t.Fatal("created source id should not be zero")
	}

	return src
}

func stringPtr(s string) *string {
	return &s
}

func createTestUser(t *testing.T, db *gorm.DB, userID int64, email string) {
	t.Helper()

	ctx := context.Background()
	now := time.Now().UTC()

	u := &user.User{
		ID:           userID,
		Email:        email,
		PasswordHash: "$2a$10$placeholder",
		DisplayName:  "Test User",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := db.WithContext(ctx).Create(u).Error; err != nil {
		t.Fatalf("create test user: %v", err)
	}
}

func runMigrations(t *testing.T, db *sql.DB) error {
	t.Helper()

	driver, err := migratepostgres.WithInstance(db, &migratepostgres.Config{})
	if err != nil {
		return fmt.Errorf("create migrate postgres driver: %w", err)
	}

	migrationsPath := migrationsDir(t)

	m, err := migrate.NewWithDatabaseInstance(
		"file://"+migrationsPath,
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

func migrationsDir(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("get current file path")
	}

	// current file:
	// internal/module/source/repository_integration_test.go
	//
	// project root:
	// ../../../..
	projectRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))

	return filepath.Join(projectRoot, "migrations")
}

func setupSourceRepositoryTestDB(t *testing.T) (*gorm.DB, func()) {
	t.Helper()

	ctx := context.Background()

	pgContainer, err := postgres.Run(
		ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("contentflow_test"),
		postgres.WithUsername("contentflow"),
		postgres.WithPassword("contentflow"),
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

	if err := runMigrations(t, sqlDB); err != nil {
		sqlDB.Close()
		cleanup()
		t.Fatalf("run migrations: %v", err)
	}

	db, err := gorm.Open(gormpostgres.Open(connStr), &gorm.Config{
		TranslateError: true,
	})
	if err != nil {
		sqlDB.Close()
		cleanup()
		t.Fatalf("open gorm db: %v", err)
	}

	createTestUser(t, db, 100, "user100@test.com")
	createTestUser(t, db, 200, "user200@test.com")

	return db, func() {
		gormSQLDB, err := db.DB()
		if err == nil {
			_ = gormSQLDB.Close()
		}

		_ = sqlDB.Close()
		cleanup()
	}
}
