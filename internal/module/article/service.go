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
	repo Repository
	now  func() time.Time
}

type Option func(*Service)

func WithNow(now func() time.Time) Option {
	return func(s *Service) {
		s.now = now
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
