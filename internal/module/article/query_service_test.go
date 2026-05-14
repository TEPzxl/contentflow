package article_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tepzxl/contentflow/internal/module/article"
	articlemocks "github.com/tepzxl/contentflow/internal/module/article/mocks"
	"go.uber.org/mock/gomock"
)

func TestArticleService_ListArticles(t *testing.T) {
	now := fixedTime()

	tests := []struct {
		name      string
		req       article.ListArticlesRequest
		cache     *fakeArticleListCache
		mock      func(ctx context.Context, repo *articlemocks.MockRepository)
		wantErr   error
		wantTotal int64
		wantSets  int
	}{
		{
			name: "list with normalized pagination and filters",
			req: article.ListArticlesRequest{
				UserID:   100,
				SourceID: 1,
				Query:    " Go ",
				IsRead:   boolPtr(false),
				Limit:    1000,
				Offset:   -10,
			},
			cache: &fakeArticleListCache{},
			mock: func(ctx context.Context, repo *articlemocks.MockRepository) {
				repo.EXPECT().
					ListByUser(ctx, article.ListArticlesParams{
						UserID:   100,
						SourceID: 1,
						Query:    "Go",
						IsRead:   boolPtr(false),
						Limit:    100,
						Offset:   0,
					}).
					Return([]article.ArticleWithState{sampleArticleWithState(now)}, int64(1), nil)
			},
			wantTotal: 1,
			wantSets:  1,
		},
		{
			name: "cache hit skips repository",
			req: article.ListArticlesRequest{
				UserID: 100,
			},
			cache: &fakeArticleListCache{
				resp: &article.ListArticlesResponse{
					Articles: []article.ArticleDTO{sampleArticleDTO(now)},
					Total:    1,
					Limit:    20,
					Offset:   0,
				},
			},
			mock: func(ctx context.Context, repo *articlemocks.MockRepository) {
			},
			wantTotal: 1,
			wantSets:  0,
		},
		{
			name: "repository error",
			req: article.ListArticlesRequest{
				UserID: 100,
			},
			cache: &fakeArticleListCache{},
			mock: func(ctx context.Context, repo *articlemocks.MockRepository) {
				repo.EXPECT().
					ListByUser(ctx, gomock.AssignableToTypeOf(article.ListArticlesParams{})).
					Return(nil, int64(0), errRepo)
			},
			wantErr: errRepo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()
			repo := articlemocks.NewMockRepository(ctrl)
			tt.mock(ctx, repo)

			svc := article.NewService(repo, article.WithListCache(tt.cache, time.Minute))
			resp, err := svc.ListArticles(ctx, tt.req)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("ListArticles() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ListArticles() unexpected error = %v", err)
			}
			if resp.Total != tt.wantTotal {
				t.Fatalf("Total = %d, want %d", resp.Total, tt.wantTotal)
			}
			if tt.cache.sets != tt.wantSets {
				t.Fatalf("cache sets = %d, want %d", tt.cache.sets, tt.wantSets)
			}
		})
	}
}

func TestArticleService_GetArticle(t *testing.T) {
	now := fixedTime()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	repo := articlemocks.NewMockRepository(ctrl)
	repo.EXPECT().
		FindByUserAndID(ctx, int64(100), int64(1)).
		Return(sampleArticleWithState(now), nil)

	svc := article.NewService(repo)
	resp, err := svc.GetArticle(ctx, article.GetArticleRequest{UserID: 100, ArticleID: 1})
	if err != nil {
		t.Fatalf("GetArticle() error = %v", err)
	}
	if resp.Article.ID != 1 || resp.Article.IsRead != true {
		t.Fatalf("Article = %#v", resp.Article)
	}
}

func TestArticleService_UpdateState(t *testing.T) {
	now := fixedTime()

	tests := []struct {
		name      string
		req       article.UpdateArticleStateRequest
		mock      func(ctx context.Context, repo *articlemocks.MockRepository, cache *fakeArticleListCache)
		wantRead  bool
		wantSaved bool
	}{
		{
			name: "mark read",
			req: article.UpdateArticleStateRequest{
				UserID:    100,
				ArticleID: 1,
				IsRead:    boolPtr(true),
			},
			mock: func(ctx context.Context, repo *articlemocks.MockRepository, cache *fakeArticleListCache) {
				repo.EXPECT().
					UpsertState(ctx, article.UpsertArticleStateParams{
						UserID:    100,
						ArticleID: 1,
						IsRead:    boolPtr(true),
						IsSaved:   nil,
						Now:       now,
					}).
					Return(sampleArticleWithState(now), nil)
			},
			wantRead:  true,
			wantSaved: false,
		},
		{
			name: "save article",
			req: article.UpdateArticleStateRequest{
				UserID:    100,
				ArticleID: 1,
				IsSaved:   boolPtr(true),
			},
			mock: func(ctx context.Context, repo *articlemocks.MockRepository, cache *fakeArticleListCache) {
				got := sampleArticleWithState(now)
				got.IsSaved = true
				repo.EXPECT().
					UpsertState(ctx, article.UpsertArticleStateParams{
						UserID:    100,
						ArticleID: 1,
						IsRead:    nil,
						IsSaved:   boolPtr(true),
						Now:       now,
					}).
					Return(got, nil)
			},
			wantRead:  true,
			wantSaved: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()
			repo := articlemocks.NewMockRepository(ctrl)
			cache := &fakeArticleListCache{}
			tt.mock(ctx, repo, cache)

			svc := article.NewService(
				repo,
				article.WithNow(func() time.Time { return now }),
				article.WithListCache(cache, time.Minute),
			)

			resp, err := svc.UpdateState(ctx, tt.req)
			if err != nil {
				t.Fatalf("UpdateState() error = %v", err)
			}
			if resp.Article.IsRead != tt.wantRead || resp.Article.IsSaved != tt.wantSaved {
				t.Fatalf("Article = %#v", resp.Article)
			}
			if cache.deletes != 1 {
				t.Fatalf("cache deletes = %d, want 1", cache.deletes)
			}
		})
	}
}

type fakeArticleListCache struct {
	resp    *article.ListArticlesResponse
	sets    int
	deletes int
}

func (f *fakeArticleListCache) GetList(ctx context.Context, req article.ListArticlesRequest) (*article.ListArticlesResponse, bool, error) {
	if f.resp == nil {
		return nil, false, nil
	}
	return f.resp, true, nil
}

func (f *fakeArticleListCache) SetList(ctx context.Context, req article.ListArticlesRequest, resp *article.ListArticlesResponse, ttl time.Duration) error {
	f.sets++
	return nil
}

func (f *fakeArticleListCache) DeleteUser(ctx context.Context, userID int64) error {
	f.deletes++
	return nil
}

func sampleArticleWithState(now time.Time) article.ArticleWithState {
	url := "https://example.com/a"
	return article.ArticleWithState{
		ID:          1,
		SourceID:    1,
		SourceType:  "rss",
		Title:       "Go Article",
		URL:         &url,
		Author:      "Go Team",
		Summary:     "summary",
		Content:     "content",
		PublishedAt: &now,
		CreatedAt:   now,
		UpdatedAt:   now,
		IsRead:      true,
		IsSaved:     false,
		ReadAt:      &now,
	}
}

func sampleArticleDTO(now time.Time) article.ArticleDTO {
	a := sampleArticleWithState(now)
	return article.ArticleDTO{
		ID:          a.ID,
		SourceID:    a.SourceID,
		SourceType:  a.SourceType,
		Title:       a.Title,
		URL:         a.URL,
		Author:      a.Author,
		Summary:     a.Summary,
		Content:     a.Content,
		PublishedAt: a.PublishedAt,
		CreatedAt:   a.CreatedAt,
		UpdatedAt:   a.UpdatedAt,
		IsRead:      a.IsRead,
		IsSaved:     a.IsSaved,
		ReadAt:      a.ReadAt,
		SavedAt:     a.SavedAt,
	}
}

func boolPtr(v bool) *bool {
	return &v
}
