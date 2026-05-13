package collector

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tepzxl/contentflow/internal/module/source"
)

var (
	ErrCollectorNotFound = errors.New("collector not found")
	ErrCollectionFailed  = errors.New("collection failed")
)

type ArticleWriter interface {
	SaveCollectedItems(ctx context.Context, items []CollectedItem) (*ArticleWriteResult, error)
}

type Service interface {
	CollectSource(ctx context.Context, req CollectSourceRequest) (*CollectSourceResponse, error)
}

type CollectionService struct {
	sourceRepo    source.Repository
	runRepo       RunRepository
	registry      *Registry
	articleWriter ArticleWriter
	now           func() time.Time
}

type Option func(*CollectionService)

func WithNow(now func() time.Time) Option {
	return func(cs *CollectionService) {
		cs.now = now
	}
}
func NewService(sourceRepo source.Repository, runRepo RunRepository, registry *Registry, articleWriter ArticleWriter, opts ...Option) Service {
	s := &CollectionService{
		sourceRepo:    sourceRepo,
		runRepo:       runRepo,
		registry:      registry,
		articleWriter: articleWriter,
		now:           func() time.Time { return time.Now().UTC() },
	}

	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *CollectionService) CollectSource(ctx context.Context, req CollectSourceRequest) (*CollectSourceResponse, error) {
	src, err := s.sourceRepo.FindByUserIDAndID(ctx, req.UserID, req.SourceID)
	if errors.Is(err, source.ErrSourceNotFound) {
		return nil, source.ErrSourceNotAccessible
	}

	if err != nil {
		return nil, fmt.Errorf("find source: %w", err)
	}

	c, ok := s.registry.Get(src.Type)
	if !ok {
		return nil, ErrCollectorNotFound
	}

	now := s.now()

	run := &CollectionRun{
		SourceID:        src.ID,
		Status:          RunStatusRunning,
		StartedAt:       now,
		FetchedCount:    0,
		InsertedCount:   0,
		DuplicatedCount: 0,
		ErrorMessage:    "",
		CreatedAt:       now,
	}

	if err := s.runRepo.Create(ctx, run); err != nil {
		return nil, fmt.Errorf("create collection run: %w", err)
	}

	items, err := c.Collect(ctx, src)
	if err != nil {
		return s.finishFailed(ctx, run.ID, src, 0, 0, 0, err)
	}

	writeResult, err := s.articleWriter.SaveCollectedItems(ctx, items)
	if err != nil {
		return s.finishFailed(ctx, run.ID, src, len(items), 0, 0, err)
	}

	finishedAt := s.now()

	if err := s.runRepo.Finish(ctx, FinishRunParams{
		RunID:           run.ID,
		Status:          RunStatusSuccess,
		FinishedAt:      finishedAt,
		FetchedCount:    len(items),
		InsertedCount:   writeResult.InsertedCount,
		DuplicatedCount: writeResult.DuplicatedCount,
		ErrorMessage:    "",
	}); err != nil {
		return nil, fmt.Errorf("finish collection run success: %w", err)
	}

	src.LastFetchedAt = &finishedAt
	src.LastFetchStatus = RunStatusSuccess
	src.LastFetchMessage = ""
	src.UpdatedAt = finishedAt

	if err := s.sourceRepo.Update(ctx, src); err != nil {
		return nil, fmt.Errorf("update source fetch status: %w", err)
	}

	return &CollectSourceResponse{
		RunID:           run.ID,
		SourceID:        src.ID,
		Status:          RunStatusSuccess,
		FetchedCount:    len(items),
		InsertedCount:   writeResult.InsertedCount,
		DuplicatedCount: writeResult.DuplicatedCount,
		ErrorMessage:    "",
	}, nil
}
func (s *CollectionService) finishFailed(
	ctx context.Context,
	runID int64,
	src *source.Source,
	fetchedCount int,
	insertedCount int,
	duplicatedCount int,
	cause error,
) (*CollectSourceResponse, error) {
	finishedAt := s.now()
	errorMessage := cause.Error()

	if err := s.runRepo.Finish(ctx, FinishRunParams{
		RunID:           runID,
		Status:          RunStatusFailed,
		FinishedAt:      finishedAt,
		FetchedCount:    fetchedCount,
		InsertedCount:   insertedCount,
		DuplicatedCount: duplicatedCount,
		ErrorMessage:    errorMessage,
	}); err != nil {
		return nil, fmt.Errorf("finish collection run failed: %w", err)
	}

	src.LastFetchedAt = &finishedAt
	src.LastFetchStatus = RunStatusFailed
	src.LastFetchMessage = errorMessage
	src.UpdatedAt = finishedAt

	if err := s.sourceRepo.Update(ctx, src); err != nil {
		return nil, fmt.Errorf("update source failed status: %w", err)
	}

	return &CollectSourceResponse{
		RunID:           runID,
		SourceID:        src.ID,
		Status:          RunStatusFailed,
		FetchedCount:    fetchedCount,
		InsertedCount:   insertedCount,
		DuplicatedCount: duplicatedCount,
		ErrorMessage:    errorMessage,
	}, fmt.Errorf("%w: %v", ErrCollectionFailed, cause)
}
