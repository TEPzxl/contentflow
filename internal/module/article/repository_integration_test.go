//go:build integration

package article_test

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
	"github.com/tepzxl/contentflow/internal/module/article"
	"github.com/tepzxl/contentflow/internal/module/source"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestArticleRepository_CreateIfNotExists_Deduplication(t *testing.T) {
	tests := []struct {
		name       string
		second     func(first *article.Article) *article.Article
		wantSecond bool
	}{
		{
			name: "same source and same external id should not create",
			second: func(first *article.Article) *article.Article {
				a := sampleArticle(first.SourceID)
				a.ContentHash = "different-hash"
				return a
			},
			wantSecond: false,
		},
		{
			name: "same source and same content hash should not create",
			second: func(first *article.Article) *article.Article {
				a := sampleArticle(first.SourceID)
				a.ExternalID = stringPtr("different-guid")
				a.ContentHash = first.ContentHash
				return a
			},
			wantSecond: false,
		},
		{
			name: "nil external id should deduplicate by content hash",
			second: func(first *article.Article) *article.Article {
				a := sampleArticle(first.SourceID)
				a.ExternalID = nil
				a.ContentHash = first.ContentHash
				return a
			},
			wantSecond: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, cleanup := setupArticleRepositoryTestDB(t)
			defer cleanup()

			userID := createTestUser(t, db, "u1@example.com")
			sourceID := createTestSource(t, db, userID, "https://example.com/feed.xml")

			repo := article.NewRepository(db)
			ctx := context.Background()

			first := sampleArticle(sourceID)

			if tt.name == "nil external id should deduplicate by content hash" {
				first.ExternalID = nil
			}

			created, err := repo.CreateIfNotExists(ctx, first)
			if err != nil {
				t.Fatalf("first CreateIfNotExists() error = %v", err)
			}
			if !created {
				t.Fatal("first created = false, want true")
			}

			second := tt.second(first)

			created, err = repo.CreateIfNotExists(ctx, second)
			if err != nil {
				t.Fatalf("second CreateIfNotExists() error = %v", err)
			}

			if created != tt.wantSecond {
				t.Fatalf("second created = %v, want %v", created, tt.wantSecond)
			}
		})
	}
}

func TestArticleRepository_ListByUserAndState(t *testing.T) {
	db, cleanup := setupArticleRepositoryTestDB(t)
	defer cleanup()

	ctx := context.Background()
	repo := article.NewRepository(db)

	userID := createTestUser(t, db, "articles-user@example.com")
	otherUserID := createTestUser(t, db, "articles-other@example.com")
	sourceID := createTestSource(t, db, userID, "https://example.com/feed.xml")
	otherSourceID := createTestSource(t, db, otherUserID, "https://other.example.com/feed.xml")

	first := sampleArticle(sourceID)
	first.Title = "Go Article"
	first.ContentHash = "hash-list-1"
	first.ExternalID = stringPtr("guid-list-1")
	second := sampleArticle(sourceID)
	second.Title = "Database Article"
	second.ContentHash = "hash-list-2"
	second.ExternalID = stringPtr("guid-list-2")
	other := sampleArticle(otherSourceID)
	other.Title = "Other User Article"
	other.ContentHash = "hash-list-3"
	other.ExternalID = stringPtr("guid-list-3")

	if created, err := repo.CreateIfNotExists(ctx, first); err != nil || !created {
		t.Fatalf("create first article = %v, %v", created, err)
	}
	if created, err := repo.CreateIfNotExists(ctx, second); err != nil || !created {
		t.Fatalf("create second article = %v, %v", created, err)
	}
	if created, err := repo.CreateIfNotExists(ctx, other); err != nil || !created {
		t.Fatalf("create other article = %v, %v", created, err)
	}

	now := time.Now().UTC()
	updated, err := repo.UpsertState(ctx, article.UpsertArticleStateParams{
		UserID:    userID,
		ArticleID: first.ID,
		IsRead:    boolPtr(true),
		IsSaved:   boolPtr(true),
		Now:       now,
	})
	if err != nil {
		t.Fatalf("UpsertState() error = %v", err)
	}
	if !updated.IsRead || !updated.IsSaved || updated.ReadAt == nil || updated.SavedAt == nil {
		t.Fatalf("updated state = %#v", updated)
	}

	list, total, err := repo.ListByUser(ctx, article.ListArticlesParams{
		UserID: userID,
		Limit:  20,
		Offset: 0,
	})
	if err != nil {
		t.Fatalf("ListByUser() error = %v", err)
	}
	if total != 2 || len(list) != 2 {
		t.Fatalf("total/len = %d/%d, want 2/2", total, len(list))
	}

	saved, total, err := repo.ListByUser(ctx, article.ListArticlesParams{
		UserID:  userID,
		IsSaved: boolPtr(true),
		Limit:   20,
		Offset:  0,
	})
	if err != nil {
		t.Fatalf("ListByUser(saved) error = %v", err)
	}
	if total != 1 || len(saved) != 1 || saved[0].ID != first.ID {
		t.Fatalf("saved result = total %d rows %#v", total, saved)
	}

	search, total, err := repo.ListByUser(ctx, article.ListArticlesParams{
		UserID: userID,
		Query:  "database",
		Limit:  20,
		Offset: 0,
	})
	if err != nil {
		t.Fatalf("ListByUser(search) error = %v", err)
	}
	if total != 1 || len(search) != 1 || search[0].ID != second.ID {
		t.Fatalf("search result = total %d rows %#v", total, search)
	}

	got, err := repo.FindByUserAndID(ctx, userID, second.ID)
	if err != nil {
		t.Fatalf("FindByUserAndID() error = %v", err)
	}
	if got.IsRead || got.IsSaved {
		t.Fatalf("default state = read %v saved %v, want false/false", got.IsRead, got.IsSaved)
	}

	_, err = repo.FindByUserAndID(ctx, userID, other.ID)
	if !errors.Is(err, article.ErrArticleNotFound) {
		t.Fatalf("FindByUserAndID(other) error = %v, want ErrArticleNotFound", err)
	}
}

func createTestUser(t *testing.T, db *gorm.DB, email string) int64 {
	t.Helper()

	var id int64

	err := db.Raw(`
		INSERT INTO users (email, password_hash, display_name, created_at, updated_at)
		VALUES (?, ?, ?, NOW(), NOW())
		RETURNING id
	`, email, "hashed-password", "test user").Scan(&id).Error
	if err != nil {
		t.Fatalf("create test user: %v", err)
	}

	if id == 0 {
		t.Fatal("created user id should not be zero")
	}

	return id
}

func createTestSource(t *testing.T, db *gorm.DB, userID int64, rawURL string) int64 {
	t.Helper()

	var id int64

	err := db.Raw(`
		INSERT INTO sources (
			user_id,
			name,
			type,
			url,
			config_json,
			is_active,
			last_fetch_status,
			last_fetch_message,
			created_at,
			updated_at
		)
		VALUES (?, ?, ?, ?, '{}'::jsonb, true, '', '', NOW(), NOW())
		RETURNING id
	`, userID, "Go Blog", source.TypeRSS, rawURL).Scan(&id).Error
	if err != nil {
		t.Fatalf("create test source: %v", err)
	}

	if id == 0 {
		t.Fatal("created source id should not be zero")
	}

	return id
}

func stringPtr(s string) *string {
	return &s
}

func sampleArticle(sourceID int64) *article.Article {
	now := time.Now().UTC()
	externalID := "rss-guid-1"
	rawURL := "https://go.dev/blog/1"

	return &article.Article{
		SourceID:    sourceID,
		SourceType:  source.TypeRSS,
		ExternalID:  &externalID,
		Title:       "Go Blog 1",
		URL:         &rawURL,
		OriginalURL: &rawURL,
		Author:      "Go Team",
		Summary:     "summary",
		Content:     "content",
		ContentHash: "hash-1",
		PublishedAt: &now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func setupArticleRepositoryTestDB(t *testing.T) (*gorm.DB, func()) {
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

	if err := runMigrations(t, sqlDB); err != nil {
		_ = sqlDB.Close()
		cleanup()
		t.Fatalf("run migrations: %v", err)
	}

	db, err := gorm.Open(gormpostgres.Open(connStr), &gorm.Config{
		TranslateError: true,
	})
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

func runMigrations(t *testing.T, db *sql.DB) error {
	t.Helper()

	driver, err := migratepostgres.WithInstance(db, &migratepostgres.Config{})
	if err != nil {
		return fmt.Errorf("create migrate postgres driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://"+migrationsDir(t),
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

	projectRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))

	return filepath.Join(projectRoot, "migrations")
}
