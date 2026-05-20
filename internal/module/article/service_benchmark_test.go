package article_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/tepzxl/contentflow/internal/module/article"
)

func BenchmarkService_ListArticles(b *testing.B) {
	repo := &benchmarkArticleRepository{rows: make([]article.ArticleWithState, 100)}
	for i := range repo.rows {
		repo.rows[i] = article.ArticleWithState{
			ID:         int64(i + 1),
			SourceID:   10,
			SourceType: "rss",
			Title:      fmt.Sprintf("Article %d", i+1),
			Content:    "benchmark content",
			CreatedAt:  time.Now().UTC(),
			UpdatedAt:  time.Now().UTC(),
		}
	}
	service := article.NewService(repo)
	ctx := context.Background()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := service.ListArticles(ctx, article.ListArticlesRequest{
			UserID: 100,
			Limit:  100,
		}); err != nil {
			b.Fatalf("ListArticles() error = %v", err)
		}
	}
}

type benchmarkArticleRepository struct {
	rows []article.ArticleWithState
}

func (r *benchmarkArticleRepository) CreateIfNotExists(context.Context, *article.Article) (bool, error) {
	return true, nil
}

func (r *benchmarkArticleRepository) ListByUser(context.Context, article.ListArticlesParams) ([]article.ArticleWithState, int64, error) {
	return r.rows, int64(len(r.rows)), nil
}

func (r *benchmarkArticleRepository) FindByUserAndID(context.Context, int64, int64) (article.ArticleWithState, error) {
	return article.ArticleWithState{}, article.ErrArticleNotFound
}

func (r *benchmarkArticleRepository) UpsertState(context.Context, article.UpsertArticleStateParams) (article.ArticleWithState, error) {
	return article.ArticleWithState{}, nil
}
