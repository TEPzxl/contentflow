package article_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tepzxl/contentflow/internal/module/article"
	articlemocks "github.com/tepzxl/contentflow/internal/module/article/mocks"
	"github.com/tepzxl/contentflow/internal/module/collector"
	"github.com/tepzxl/contentflow/internal/module/source"
	"go.uber.org/mock/gomock"
)

var errRepo = errors.New("repository error")

func TestArticleService_SaveCollectedItems(t *testing.T) {
	now := fixedTime()

	tests := []struct {
		name           string
		items          []collector.CollectedItem
		mock           func(t *testing.T, ctx context.Context, repo *articlemocks.MockRepository)
		wantErr        error
		wantInserted   int
		wantDuplicated int
	}{
		{
			name:  "empty items",
			items: []collector.CollectedItem{},
			mock: func(t *testing.T, ctx context.Context, repo *articlemocks.MockRepository) {
			},
			wantErr:        nil,
			wantInserted:   0,
			wantDuplicated: 0,
		},
		{
			name: "single inserted item",
			items: []collector.CollectedItem{
				sampleCollectedItem(),
			},
			mock: func(t *testing.T, ctx context.Context, repo *articlemocks.MockRepository) {
				repo.EXPECT().
					CreateIfNotExists(ctx, gomock.AssignableToTypeOf(&article.Article{})).
					DoAndReturn(func(_ context.Context, a *article.Article) (bool, error) {
						if a.SourceID != 1 {
							t.Fatalf("SourceID = %d, want 1", a.SourceID)
						}

						if a.SourceType != source.TypeRSS {
							t.Fatalf("SourceType = %s, want rss", a.SourceType)
						}

						if a.ExternalID == nil || *a.ExternalID != "rss-guid-1" {
							t.Fatalf("ExternalID = %v, want rss-guid-1", a.ExternalID)
						}

						if a.Title != "Go Blog 1" {
							t.Fatalf("Title = %s, want Go Blog 1", a.Title)
						}

						if a.URL == nil || *a.URL != "https://go.dev/blog/1" {
							t.Fatalf("URL = %v, want https://go.dev/blog/1", a.URL)
						}

						if a.OriginalURL == nil || *a.OriginalURL != "https://go.dev/blog/1?utm_source=rss" {
							t.Fatalf("OriginalURL = %v", a.OriginalURL)
						}

						if a.Author != "Go Team" {
							t.Fatalf("Author = %s, want Go Team", a.Author)
						}

						if a.Summary != "summary" {
							t.Fatalf("Summary = %s, want summary", a.Summary)
						}

						if a.Content != "content" {
							t.Fatalf("Content = %s, want content", a.Content)
						}

						if a.ContentHash != "hash-1" {
							t.Fatalf("ContentHash = %s, want hash-1", a.ContentHash)
						}

						if a.PublishedAt == nil || !a.PublishedAt.Equal(now) {
							t.Fatalf("PublishedAt = %v, want %v", a.PublishedAt, now)
						}

						if !a.CreatedAt.Equal(now) {
							t.Fatalf("CreatedAt = %v, want %v", a.CreatedAt, now)
						}

						if !a.UpdatedAt.Equal(now) {
							t.Fatalf("UpdatedAt = %v, want %v", a.UpdatedAt, now)
						}

						return true, nil
					})
			},
			wantErr:        nil,
			wantInserted:   1,
			wantDuplicated: 0,
		},
		{
			name: "single duplicated item",
			items: []collector.CollectedItem{
				sampleCollectedItem(),
			},
			mock: func(t *testing.T, ctx context.Context, repo *articlemocks.MockRepository) {
				repo.EXPECT().
					CreateIfNotExists(ctx, gomock.AssignableToTypeOf(&article.Article{})).
					Return(false, nil)
			},
			wantErr:        nil,
			wantInserted:   0,
			wantDuplicated: 1,
		},
		{
			name: "mixed inserted and duplicated items",
			items: []collector.CollectedItem{
				sampleCollectedItem(),
				func() collector.CollectedItem {
					item := sampleCollectedItem()
					item.ExternalID = strPtr("rss-guid-2")
					item.Title = "Go Blog 2"
					item.ContentHash = "hash-2"
					return item
				}(),
				func() collector.CollectedItem {
					item := sampleCollectedItem()
					item.ExternalID = strPtr("rss-guid-3")
					item.Title = "Go Blog 3"
					item.ContentHash = "hash-3"
					return item
				}(),
			},
			mock: func(t *testing.T, ctx context.Context, repo *articlemocks.MockRepository) {
				gomock.InOrder(
					repo.EXPECT().
						CreateIfNotExists(ctx, gomock.AssignableToTypeOf(&article.Article{})).
						Return(true, nil),
					repo.EXPECT().
						CreateIfNotExists(ctx, gomock.AssignableToTypeOf(&article.Article{})).
						Return(false, nil),
					repo.EXPECT().
						CreateIfNotExists(ctx, gomock.AssignableToTypeOf(&article.Article{})).
						Return(true, nil),
				)
			},
			wantErr:        nil,
			wantInserted:   2,
			wantDuplicated: 1,
		},
		{
			name: "invalid source id",
			items: []collector.CollectedItem{
				func() collector.CollectedItem {
					item := sampleCollectedItem()
					item.SourceID = 0
					return item
				}(),
			},
			mock: func(t *testing.T, ctx context.Context, repo *articlemocks.MockRepository) {
			},
			wantErr: article.ErrInvalidCollectedItem,
		},
		{
			name: "invalid source type",
			items: []collector.CollectedItem{
				func() collector.CollectedItem {
					item := sampleCollectedItem()
					item.SourceType = "   "
					return item
				}(),
			},
			mock: func(t *testing.T, ctx context.Context, repo *articlemocks.MockRepository) {
			},
			wantErr: article.ErrInvalidCollectedItem,
		},
		{
			name: "invalid title",
			items: []collector.CollectedItem{
				func() collector.CollectedItem {
					item := sampleCollectedItem()
					item.Title = "   "
					return item
				}(),
			},
			mock: func(t *testing.T, ctx context.Context, repo *articlemocks.MockRepository) {
			},
			wantErr: article.ErrInvalidCollectedItem,
		},
		{
			name: "invalid content hash",
			items: []collector.CollectedItem{
				func() collector.CollectedItem {
					item := sampleCollectedItem()
					item.ContentHash = "   "
					return item
				}(),
			},
			mock: func(t *testing.T, ctx context.Context, repo *articlemocks.MockRepository) {
			},
			wantErr: article.ErrInvalidCollectedItem,
		},
		{
			name: "repository error",
			items: []collector.CollectedItem{
				sampleCollectedItem(),
			},
			mock: func(t *testing.T, ctx context.Context, repo *articlemocks.MockRepository) {
				repo.EXPECT().
					CreateIfNotExists(ctx, gomock.AssignableToTypeOf(&article.Article{})).
					Return(false, errRepo)
			},
			wantErr: errRepo,
		},
		{
			name: "normalize optional string fields",
			items: []collector.CollectedItem{
				func() collector.CollectedItem {
					item := sampleCollectedItem()

					externalID := "  rss-guid-1  "
					url := "  https://go.dev/blog/1  "
					originalURL := "   "

					item.ExternalID = &externalID
					item.URL = &url
					item.OriginalURL = &originalURL
					item.Author = "  Go Team  "
					item.Summary = "  summary  "
					item.Content = "  content  "
					item.Title = "  Go Blog 1  "
					item.ContentHash = "  hash-1  "

					return item
				}(),
			},
			mock: func(t *testing.T, ctx context.Context, repo *articlemocks.MockRepository) {
				repo.EXPECT().
					CreateIfNotExists(ctx, gomock.AssignableToTypeOf(&article.Article{})).
					DoAndReturn(func(_ context.Context, a *article.Article) (bool, error) {
						if a.ExternalID == nil || *a.ExternalID != "rss-guid-1" {
							t.Fatalf("ExternalID = %v, want rss-guid-1", a.ExternalID)
						}

						if a.URL == nil || *a.URL != "https://go.dev/blog/1" {
							t.Fatalf("URL = %v, want https://go.dev/blog/1", a.URL)
						}

						if a.OriginalURL != nil {
							t.Fatalf("OriginalURL = %v, want nil", a.OriginalURL)
						}

						if a.Author != "Go Team" {
							t.Fatalf("Author = %q, want Go Team", a.Author)
						}

						if a.Summary != "summary" {
							t.Fatalf("Summary = %q, want summary", a.Summary)
						}

						if a.Content != "content" {
							t.Fatalf("Content = %q, want content", a.Content)
						}

						if a.Title != "Go Blog 1" {
							t.Fatalf("Title = %q, want Go Blog 1", a.Title)
						}

						if a.ContentHash != "hash-1" {
							t.Fatalf("ContentHash = %q, want hash-1", a.ContentHash)
						}

						return true, nil
					})
			},
			wantErr:        nil,
			wantInserted:   1,
			wantDuplicated: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()
			repo := articlemocks.NewMockRepository(ctrl)

			tt.mock(t, ctx, repo)

			svc := article.NewService(
				repo,
				article.WithNow(func() time.Time {
					return now
				}),
			)

			result, err := svc.SaveCollectedItems(ctx, tt.items)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("SaveCollectedItems() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("SaveCollectedItems() unexpected error = %v", err)
			}

			if result == nil {
				t.Fatal("SaveCollectedItems() result is nil")
			}

			if result.InsertedCount != tt.wantInserted {
				t.Fatalf("InsertedCount = %d, want %d", result.InsertedCount, tt.wantInserted)
			}

			if result.DuplicatedCount != tt.wantDuplicated {
				t.Fatalf("DuplicatedCount = %d, want %d", result.DuplicatedCount, tt.wantDuplicated)
			}
		})
	}
}
func strPtr(s string) *string {
	return &s
}

func fixedTime() time.Time {
	return time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
}

func sampleCollectedItem() collector.CollectedItem {
	externalID := "rss-guid-1"
	url := "https://go.dev/blog/1"
	originalURL := "https://go.dev/blog/1?utm_source=rss"
	publishedAt := fixedTime()

	return collector.CollectedItem{
		SourceID:    1,
		SourceType:  source.TypeRSS,
		ExternalID:  &externalID,
		Title:       "Go Blog 1",
		URL:         &url,
		OriginalURL: &originalURL,
		Author:      "Go Team",
		Summary:     "summary",
		Content:     "content",
		ContentHash: "hash-1",
		PublishedAt: &publishedAt,
	}
}
