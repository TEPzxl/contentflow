//go:build integration

package collector_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	migratepostgres "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/tepzxl/contentflow/internal/module/article"
	"github.com/tepzxl/contentflow/internal/module/collector"
	rsscollector "github.com/tepzxl/contentflow/internal/module/collector/rss"
	"github.com/tepzxl/contentflow/internal/module/source"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/datatypes"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestRSSCollectionEndToEnd(t *testing.T) {
	db, cleanup := setupCollectorIntegrationDB(t)
	defer cleanup()

	feedURL := "https://example.com/feed.xml"
	feed := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Example Feed</title>
    <link>https://example.com</link>
    <description>Example feed</description>
    <item>
      <guid>rss-guid-1</guid>
      <title>First item</title>
      <link>https://example.com/articles/1</link>
      <description>First summary</description>
      <pubDate>Wed, 13 May 2026 10:00:00 GMT</pubDate>
    </item>
    <item>
      <guid>rss-guid-2</guid>
      <title>Second item</title>
      <link>https://example.com/articles/2</link>
      <description>Second summary</description>
      <pubDate>Wed, 13 May 2026 11:00:00 GMT</pubDate>
    </item>
  </channel>
</rss>`

	ctx := context.Background()
	userID := createCollectorIntegrationUser(t, db, "rss-e2e@example.com")
	sourceRepo := source.NewRepository(db)
	src := &source.Source{
		UserID:           userID,
		Name:             "Example Feed",
		Type:             source.TypeRSS,
		URL:              &feedURL,
		ConfigJSON:       datatypes.JSON([]byte(`{}`)),
		IsActive:         true,
		LastFetchStatus:  "",
		LastFetchMessage: "",
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	if err := sourceRepo.Create(ctx, src); err != nil {
		t.Fatalf("create source: %v", err)
	}

	registry, err := collector.NewRegistry(rsscollector.NewCollector(
		rsscollector.WithFetcher(staticFeedFetcher{body: feed}),
	))
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}

	service := collector.NewService(
		sourceRepo,
		collector.NewRunRepository(db),
		registry,
		article.NewService(article.NewRepository(db)),
	)

	resp, err := service.CollectSource(ctx, collector.CollectSourceRequest{
		UserID:   userID,
		SourceID: src.ID,
	})
	if err != nil {
		t.Fatalf("CollectSource() error = %v", err)
	}

	if resp.Status != collector.RunStatusSuccess {
		t.Fatalf("Status = %q, want %q", resp.Status, collector.RunStatusSuccess)
	}
	if resp.FetchedCount != 2 || resp.InsertedCount != 2 || resp.DuplicatedCount != 0 {
		t.Fatalf("counts = fetched:%d inserted:%d duplicated:%d, want 2/2/0",
			resp.FetchedCount,
			resp.InsertedCount,
			resp.DuplicatedCount,
		)
	}

	var articleCount int64
	if err := db.Model(&article.Article{}).Where("source_id = ?", src.ID).Count(&articleCount).Error; err != nil {
		t.Fatalf("count articles: %v", err)
	}
	if articleCount != 2 {
		t.Fatalf("article count = %d, want 2", articleCount)
	}

	gotSource, err := sourceRepo.FindByUserIDAndID(ctx, userID, src.ID)
	if err != nil {
		t.Fatalf("find source after collection: %v", err)
	}
	if gotSource.LastFetchedAt == nil {
		t.Fatal("LastFetchedAt is nil")
	}
	if gotSource.LastFetchStatus != collector.RunStatusSuccess {
		t.Fatalf("LastFetchStatus = %q, want %q", gotSource.LastFetchStatus, collector.RunStatusSuccess)
	}
}

func TestCollectionFinalizationRollsBackArticleWritesWhenSourceStatusUpdateFails(t *testing.T) {
	db, cleanup := setupCollectorIntegrationDB(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Date(2026, 5, 13, 10, 0, 0, 0, time.UTC)
	feedURL := "https://example.com/rollback-feed.xml"
	userID := createCollectorIntegrationUser(t, db, "collector-rollback@example.com")
	sourceRepo := source.NewRepository(db)
	src := &source.Source{
		UserID:           userID,
		Name:             "Rollback Feed",
		Type:             source.TypeRSS,
		URL:              &feedURL,
		ConfigJSON:       datatypes.JSON([]byte(`{}`)),
		IsActive:         true,
		LastFetchStatus:  "",
		LastFetchMessage: "",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := sourceRepo.Create(ctx, src); err != nil {
		t.Fatalf("create source: %v", err)
	}

	externalID := "rollback-guid-1"
	articleURL := "https://example.com/articles/rollback-1"
	registry, err := collector.NewRegistry(fakeCollector{
		sourceType: source.TypeRSS,
		items: []collector.CollectedItem{
			{
				UserID:      userID,
				SourceID:    src.ID,
				SourceType:  source.TypeRSS,
				ExternalID:  &externalID,
				Title:       "Rollback item",
				URL:         &articleURL,
				Summary:     "summary",
				Content:     "content",
				ContentHash: "rollback-hash-1",
				PublishedAt: &now,
			},
		},
	})
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}

	articleService := article.NewService(article.NewRepository(db), article.WithNow(func() time.Time { return now }))
	writer := sourceDeletingArticleWriter{
		delegate:   articleService,
		sourceRepo: sourceRepo,
		userID:     userID,
		sourceID:   src.ID,
		deletedAt:  now.Add(time.Minute),
	}
	service := collector.NewService(
		sourceRepo,
		collector.NewRunRepository(db),
		registry,
		writer,
		collector.WithNow(func() time.Time { return now }),
		collector.WithTransactionRunner(collector.NewGormTransactionRunner(db)),
	)

	resp, err := service.CollectSource(ctx, collector.CollectSourceRequest{UserID: userID, SourceID: src.ID})
	if err == nil {
		t.Fatalf("CollectSource() error is nil, response = %#v", resp)
	}

	var articleCount int64
	if err := db.Model(&article.Article{}).Where("source_id = ?", src.ID).Count(&articleCount).Error; err != nil {
		t.Fatalf("count articles: %v", err)
	}
	if articleCount != 0 {
		t.Fatalf("article count = %d, want 0 after finalization rollback", articleCount)
	}

	var run collector.CollectionRun
	if err := db.Where("source_id = ?", src.ID).First(&run).Error; err != nil {
		t.Fatalf("find collection run: %v", err)
	}
	if run.Status != collector.RunStatusRunning {
		t.Fatalf("run status = %q, want %q after finalization rollback", run.Status, collector.RunStatusRunning)
	}

	gotSource, err := sourceRepo.FindByUserIDAndID(ctx, userID, src.ID)
	if err != nil {
		t.Fatalf("find source after rollback: %v", err)
	}
	if gotSource.DeletedAt != nil {
		t.Fatalf("source deleted_at = %v, want nil after rollback", gotSource.DeletedAt)
	}
	if gotSource.LastFetchStatus != "" {
		t.Fatalf("LastFetchStatus = %q, want empty after rollback", gotSource.LastFetchStatus)
	}
}

type sourceDeletingArticleWriter struct {
	delegate   collector.ArticleWriter
	sourceRepo source.Repository
	userID     int64
	sourceID   int64
	deletedAt  time.Time
}

func (w sourceDeletingArticleWriter) SaveCollectedItems(ctx context.Context, items []collector.CollectedItem) (*collector.ArticleWriteResult, error) {
	result, err := w.delegate.SaveCollectedItems(ctx, items)
	if err != nil {
		return nil, err
	}
	if err := w.sourceRepo.SoftDelete(ctx, w.userID, w.sourceID, w.deletedAt); err != nil {
		return nil, err
	}
	return result, nil
}

type staticFeedFetcher struct {
	body string
}

func (f staticFeedFetcher) Fetch(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(f.body)), nil
}

func setupCollectorIntegrationDB(t *testing.T) (*gorm.DB, func()) {
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

	if err := runCollectorIntegrationMigrations(t, sqlDB); err != nil {
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

func runCollectorIntegrationMigrations(t *testing.T, db *sql.DB) error {
	t.Helper()

	driver, err := migratepostgres.WithInstance(db, &migratepostgres.Config{})
	if err != nil {
		return fmt.Errorf("create migrate postgres driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://"+collectorIntegrationMigrationsDir(t),
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

func collectorIntegrationMigrationsDir(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("get current file path")
	}

	projectRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))

	return filepath.Join(projectRoot, "migrations")
}

func createCollectorIntegrationUser(t *testing.T, db *gorm.DB, email string) int64 {
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
