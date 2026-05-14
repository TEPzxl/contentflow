package article

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tepzxl/contentflow/internal/module/collector"
)

var (
	ErrInvalidCollectedItem = errors.New("invalid collected item")
)

type Service struct {
	repo      Repository
	now       func() time.Time
	listCache ListCache
	cacheTTL  time.Duration
}

type Option func(*Service)

func WithNow(now func() time.Time) Option {
	return func(s *Service) {
		s.now = now
	}
}

type ListCache interface {
	GetList(ctx context.Context, req ListArticlesRequest) (*ListArticlesResponse, bool, error)
	SetList(ctx context.Context, req ListArticlesRequest, resp *ListArticlesResponse, ttl time.Duration) error
	DeleteUser(ctx context.Context, userID int64) error
}

func WithListCache(cache ListCache, ttl time.Duration) Option {
	return func(s *Service) {
		if cache != nil && ttl > 0 {
			s.listCache = cache
			s.cacheTTL = ttl
		}
	}
}

func NewService(repo Repository, opts ...Option) *Service {
	s := &Service{
		repo: repo,
		now:  time.Now,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Service) SaveCollectedItems(ctx context.Context, items []collector.CollectedItem) (*collector.ArticleWriteResult, error) {
	result := &collector.ArticleWriteResult{}

	for _, item := range items {
		article, err := s.toArticle(item)
		if err != nil {
			return nil, err
		}

		created, err := s.repo.CreateIfNotExists(ctx, article)
		if err != nil {
			return nil, fmt.Errorf("save collected item: %w", err)
		}

		if created {
			result.InsertedCount++
		} else {
			result.DuplicatedCount++
		}
	}
	return result, nil
}

func (s *Service) ListArticles(ctx context.Context, req ListArticlesRequest) (*ListArticlesResponse, error) {
	limit := normalizeLimit(req.Limit)
	offset := normalizeOffset(req.Offset)
	normalizedReq := ListArticlesRequest{
		UserID:   req.UserID,
		SourceID: req.SourceID,
		Query:    strings.TrimSpace(req.Query),
		IsRead:   req.IsRead,
		IsSaved:  req.IsSaved,
		Limit:    limit,
		Offset:   offset,
	}

	if s.listCache != nil {
		if cached, ok, err := s.listCache.GetList(ctx, normalizedReq); err == nil && ok {
			return cached, nil
		}
	}

	rows, total, err := s.repo.ListByUser(ctx, ListArticlesParams{
		UserID:   normalizedReq.UserID,
		SourceID: normalizedReq.SourceID,
		Query:    normalizedReq.Query,
		IsRead:   normalizedReq.IsRead,
		IsSaved:  normalizedReq.IsSaved,
		Limit:    normalizedReq.Limit,
		Offset:   normalizedReq.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list articles: %w", err)
	}

	articles := make([]ArticleDTO, 0, len(rows))
	for _, row := range rows {
		articles = append(articles, toArticleDTO(row))
	}

	resp := &ListArticlesResponse{
		Articles: articles,
		Total:    total,
		Limit:    normalizedReq.Limit,
		Offset:   normalizedReq.Offset,
	}

	if s.listCache != nil {
		_ = s.listCache.SetList(ctx, normalizedReq, resp, s.cacheTTL)
	}

	return resp, nil
}

func (s *Service) GetArticle(ctx context.Context, req GetArticleRequest) (*GetArticleResponse, error) {
	row, err := s.repo.FindByUserAndID(ctx, req.UserID, req.ArticleID)
	if err != nil {
		return nil, err
	}

	return &GetArticleResponse{
		Article: toArticleDTO(row),
	}, nil
}

func (s *Service) UpdateState(ctx context.Context, req UpdateArticleStateRequest) (*UpdateArticleStateResponse, error) {
	row, err := s.repo.UpsertState(ctx, UpsertArticleStateParams{
		UserID:    req.UserID,
		ArticleID: req.ArticleID,
		IsRead:    req.IsRead,
		IsSaved:   req.IsSaved,
		Now:       s.now(),
	})
	if err != nil {
		return nil, err
	}

	if s.listCache != nil {
		_ = s.listCache.DeleteUser(ctx, req.UserID)
	}

	return &UpdateArticleStateResponse{
		Article: toArticleDTO(row),
	}, nil
}

func (s *Service) toArticle(item collector.CollectedItem) (*Article, error) {
	if item.SourceID <= 0 {
		return nil, ErrInvalidCollectedItem
	}

	sourceType := strings.TrimSpace(item.SourceType)
	if sourceType == "" {
		return nil, ErrInvalidCollectedItem
	}

	title := strings.TrimSpace(item.Title)
	if title == "" {
		return nil, ErrInvalidCollectedItem
	}

	contentHash := strings.TrimSpace(item.ContentHash)
	if contentHash == "" {
		return nil, ErrInvalidCollectedItem
	}

	now := s.now()

	return &Article{
		SourceID:    item.SourceID,
		SourceType:  sourceType,
		ExternalID:  normalizeOptionalString(item.ExternalID),
		Title:       title,
		URL:         normalizeOptionalString(item.URL),
		OriginalURL: normalizeOptionalString(item.OriginalURL),
		Author:      strings.TrimSpace(item.Author),
		Summary:     strings.TrimSpace(item.Summary),
		Content:     strings.TrimSpace(item.Content),
		ContentHash: contentHash,
		PublishedAt: item.PublishedAt,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func normalizeOptionalString(value *string) *string {
	if value == nil {
		return nil
	}

	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}

	return &trimmed
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func normalizeOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

func toArticleDTO(row ArticleWithState) ArticleDTO {
	return ArticleDTO{
		ID:          row.ID,
		SourceID:    row.SourceID,
		SourceType:  row.SourceType,
		ExternalID:  row.ExternalID,
		Title:       row.Title,
		URL:         row.URL,
		OriginalURL: row.OriginalURL,
		Author:      row.Author,
		Summary:     row.Summary,
		Content:     row.Content,
		PublishedAt: row.PublishedAt,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
		IsRead:      row.IsRead,
		IsSaved:     row.IsSaved,
		ReadAt:      row.ReadAt,
		SavedAt:     row.SavedAt,
	}
}
