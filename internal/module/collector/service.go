package collector

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/tepzxl/contentflow/internal/module/source"
)

var (
	ErrCollectorNotFound     = errors.New("collector not found")
	ErrCollectionFailed      = errors.New("collection failed")
	ErrCollectionInProgress  = errors.New("collection in progress")
	defaultCollectionLockTTL = 10 * time.Minute
	defaultStaleRunTimeout   = 30 * time.Minute
)

type ArticleWriter interface {
	SaveCollectedItems(ctx context.Context, items []CollectedItem) (*ArticleWriteResult, error)
}

type Service interface {
	CollectSource(ctx context.Context, req CollectSourceRequest) (*CollectSourceResponse, error)
	ListCollectionRuns(ctx context.Context, req ListCollectionRunsRequest) (*ListCollectionRunsResponse, error)
	GetCollectionRun(ctx context.Context, req GetCollectionRunRequest) (*GetCollectionRunResponse, error)
}

type CollectionObservation struct {
	RunID           int64
	SourceID        int64
	SourceType      string
	Status          string
	FetchedCount    int
	InsertedCount   int
	DuplicatedCount int
	Duration        time.Duration
	ErrorMessage    string
}

type CollectionObserver interface {
	ObserveCollection(ctx context.Context, observation CollectionObservation)
}

type CollectionLockReleaseFunc func(ctx context.Context) error

type CollectionLock interface {
	Acquire(ctx context.Context, sourceID int64, ttl time.Duration) (CollectionLockReleaseFunc, bool, error)
}

type SourceListCacheInvalidator interface {
	DeleteUser(ctx context.Context, userID int64) error
}

type CollectionService struct {
	sourceRepo                 source.Repository
	runRepo                    RunRepository
	registry                   *Registry
	articleWriter              ArticleWriter
	observer                   CollectionObserver
	lock                       CollectionLock
	sourceListCacheInvalidator SourceListCacheInvalidator
	transactionRunner          TransactionRunner
	lockTTL                    time.Duration
	staleRunTimeout            time.Duration
	logger                     *slog.Logger
	now                        func() time.Time
}

type Option func(*CollectionService)

func WithNow(now func() time.Time) Option {
	return func(cs *CollectionService) {
		cs.now = now
	}
}

func WithObserver(observer CollectionObserver) Option {
	return func(cs *CollectionService) {
		cs.observer = observer
	}
}

func WithLogger(logger *slog.Logger) Option {
	return func(cs *CollectionService) {
		if logger != nil {
			cs.logger = logger
		}
	}
}

func WithCollectionLock(lock CollectionLock) Option {
	return func(cs *CollectionService) {
		cs.lock = lock
	}
}

func WithCollectionLockTTL(ttl time.Duration) Option {
	return func(cs *CollectionService) {
		if ttl > 0 {
			cs.lockTTL = ttl
		}
	}
}

func WithStaleRunTimeout(timeout time.Duration) Option {
	return func(cs *CollectionService) {
		if timeout >= 0 {
			cs.staleRunTimeout = timeout
		}
	}
}

func WithSourceListCacheInvalidator(invalidator SourceListCacheInvalidator) Option {
	return func(cs *CollectionService) {
		cs.sourceListCacheInvalidator = invalidator
	}
}

func WithTransactionRunner(runner TransactionRunner) Option {
	return func(cs *CollectionService) {
		cs.transactionRunner = runner
	}
}

func NewService(sourceRepo source.Repository, runRepo RunRepository, registry *Registry, articleWriter ArticleWriter, opts ...Option) Service {
	s := &CollectionService{
		sourceRepo:      sourceRepo,
		runRepo:         runRepo,
		registry:        registry,
		articleWriter:   articleWriter,
		logger:          slog.Default(),
		lockTTL:         defaultCollectionLockTTL,
		staleRunTimeout: defaultStaleRunTimeout,
		now:             func() time.Time { return time.Now().UTC() },
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

	if err := s.markStaleRunningRunsFailed(ctx, src); err != nil {
		return nil, err
	}

	c, ok := s.registry.Get(src.Type)
	if !ok {
		return nil, ErrCollectorNotFound
	}

	release, acquired, err := s.acquireLock(ctx, src.ID)
	if err != nil {
		return nil, err
	}
	if !acquired {
		return nil, ErrCollectionInProgress
	}
	if release != nil {
		defer func() {
			if err := release(context.Background()); err != nil {
				s.logger.Warn("release collection lock failed",
					slog.Int64("source_id", src.ID),
					slog.String("error", err.Error()),
				)
			}
		}()
	}

	now := s.now()
	startedAt := now

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
		return s.finishFailed(ctx, run.ID, src, startedAt, 0, 0, 0, err)
	}

	writeResult, finishedAt, failedResp, err := s.finishSuccess(ctx, run.ID, src, startedAt, items)
	if err != nil {
		return failedResp, err
	}

	s.invalidateSourceListCache(ctx, src.UserID)

	s.observe(ctx, CollectionObservation{
		RunID:           run.ID,
		SourceID:        src.ID,
		SourceType:      src.Type,
		Status:          RunStatusSuccess,
		FetchedCount:    len(items),
		InsertedCount:   writeResult.InsertedCount,
		DuplicatedCount: writeResult.DuplicatedCount,
		Duration:        finishedAt.Sub(startedAt),
	})

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

func (s *CollectionService) markStaleRunningRunsFailed(ctx context.Context, src *source.Source) error {
	if s.staleRunTimeout <= 0 {
		return nil
	}

	staleRepo, ok := s.runRepo.(StaleRunRepository)
	if !ok {
		return nil
	}

	now := s.now()
	errorMessage := fmt.Sprintf("collection run timed out after %s", s.staleRunTimeout)
	var marked int64
	if err := s.runInFinalizationTransaction(ctx, func(txCtx context.Context) error {
		count, err := staleRepo.MarkStaleRunningFailed(txCtx, MarkStaleRunningRunsParams{
			SourceID:      src.ID,
			StartedBefore: now.Add(-s.staleRunTimeout),
			FinishedAt:    now,
			ErrorMessage:  errorMessage,
		})
		if err != nil {
			return err
		}
		marked = count
		if marked == 0 {
			return nil
		}

		src.LastFetchedAt = &now
		src.LastFetchStatus = RunStatusFailed
		src.LastFetchMessage = errorMessage
		src.UpdatedAt = now
		if err := s.sourceRepo.Update(txCtx, src); err != nil {
			return fmt.Errorf("update source stale run status: %w", err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("mark stale collection runs failed: %w", err)
	}

	if marked > 0 {
		s.invalidateSourceListCache(ctx, src.UserID)
		s.logger.Warn("marked stale collection runs failed",
			slog.Int64("source_id", src.ID),
			slog.Int64("stale_runs", marked),
			slog.Duration("timeout", s.staleRunTimeout),
		)
	}
	return nil
}

func (s *CollectionService) acquireLock(ctx context.Context, sourceID int64) (CollectionLockReleaseFunc, bool, error) {
	if s.lock == nil {
		return nil, true, nil
	}
	release, acquired, err := s.lock.Acquire(ctx, sourceID, s.lockTTL)
	if err != nil {
		return nil, false, fmt.Errorf("acquire collection lock: %w", err)
	}
	return release, acquired, nil
}

func (s *CollectionService) ListCollectionRuns(ctx context.Context, req ListCollectionRunsRequest) (*ListCollectionRunsResponse, error) {
	src, err := s.sourceRepo.FindByUserIDAndID(ctx, req.UserID, req.SourceID)
	if errors.Is(err, source.ErrSourceNotFound) {
		return nil, source.ErrSourceNotAccessible
	}
	if err != nil {
		return nil, fmt.Errorf("find source: %w", err)
	}

	limit := normalizeCollectionRunLimit(req.Limit)
	offset := normalizeCollectionRunOffset(req.Offset)

	runs, total, err := s.runRepo.ListBySourceID(ctx, ListRunsParams{
		SourceID: src.ID,
		Status:   req.Status,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list collection runs: %w", err)
	}

	return &ListCollectionRunsResponse{
		Runs:   toCollectionRunDTOs(runs),
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}, nil
}

func (s *CollectionService) GetCollectionRun(ctx context.Context, req GetCollectionRunRequest) (*GetCollectionRunResponse, error) {
	run, err := s.runRepo.FindByUserIDAndID(ctx, req.UserID, req.RunID)
	if errors.Is(err, ErrCollectionRunNotFound) {
		return nil, ErrCollectionRunNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find collection run: %w", err)
	}

	return &GetCollectionRunResponse{
		Run: toCollectionRunDTO(*run),
	}, nil
}

func (s *CollectionService) finishSuccess(
	ctx context.Context,
	runID int64,
	src *source.Source,
	startedAt time.Time,
	items []CollectedItem,
) (*ArticleWriteResult, time.Time, *CollectSourceResponse, error) {
	var writeResult *ArticleWriteResult
	var finishedAt time.Time
	var articleWriteErr error

	err := s.runInFinalizationTransaction(ctx, func(txCtx context.Context) error {
		var err error
		writeResult, err = s.articleWriter.SaveCollectedItems(txCtx, items)
		if err != nil {
			articleWriteErr = err
			return err
		}

		finishedAt = s.now()
		if err := s.runRepo.Finish(txCtx, FinishRunParams{
			RunID:           runID,
			Status:          RunStatusSuccess,
			FinishedAt:      finishedAt,
			FetchedCount:    len(items),
			InsertedCount:   writeResult.InsertedCount,
			DuplicatedCount: writeResult.DuplicatedCount,
			ErrorMessage:    "",
		}); err != nil {
			return fmt.Errorf("finish collection run success: %w", err)
		}

		src.LastFetchedAt = &finishedAt
		src.LastFetchStatus = RunStatusSuccess
		src.LastFetchMessage = ""
		src.UpdatedAt = finishedAt

		if err := s.sourceRepo.Update(txCtx, src); err != nil {
			return fmt.Errorf("update source fetch status: %w", err)
		}
		return nil
	})
	if articleWriteErr != nil {
		failedResp, failedErr := s.finishFailed(ctx, runID, src, startedAt, len(items), 0, 0, articleWriteErr)
		return nil, time.Time{}, failedResp, failedErr
	}
	if err != nil {
		return nil, time.Time{}, nil, err
	}

	return writeResult, finishedAt, nil, nil
}

func (s *CollectionService) finishFailed(
	ctx context.Context,
	runID int64,
	src *source.Source,
	startedAt time.Time,
	fetchedCount int,
	insertedCount int,
	duplicatedCount int,
	cause error,
) (*CollectSourceResponse, error) {
	finishedAt := s.now()
	errorMessage := cause.Error()

	if err := s.runInFinalizationTransaction(ctx, func(txCtx context.Context) error {
		if err := s.runRepo.Finish(txCtx, FinishRunParams{
			RunID:           runID,
			Status:          RunStatusFailed,
			FinishedAt:      finishedAt,
			FetchedCount:    fetchedCount,
			InsertedCount:   insertedCount,
			DuplicatedCount: duplicatedCount,
			ErrorMessage:    errorMessage,
		}); err != nil {
			return fmt.Errorf("finish collection run failed: %w", err)
		}

		src.LastFetchedAt = &finishedAt
		src.LastFetchStatus = RunStatusFailed
		src.LastFetchMessage = errorMessage
		src.UpdatedAt = finishedAt

		if err := s.sourceRepo.Update(txCtx, src); err != nil {
			return fmt.Errorf("update source failed status: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	s.invalidateSourceListCache(ctx, src.UserID)

	s.observe(ctx, CollectionObservation{
		RunID:           runID,
		SourceID:        src.ID,
		SourceType:      src.Type,
		Status:          RunStatusFailed,
		FetchedCount:    fetchedCount,
		InsertedCount:   insertedCount,
		DuplicatedCount: duplicatedCount,
		Duration:        finishedAt.Sub(startedAt),
		ErrorMessage:    errorMessage,
	})

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

func (s *CollectionService) runInFinalizationTransaction(ctx context.Context, fn func(context.Context) error) error {
	if s.transactionRunner == nil {
		return fn(ctx)
	}
	return s.transactionRunner.RunInTransaction(ctx, fn)
}

func (s *CollectionService) invalidateSourceListCache(ctx context.Context, userID int64) {
	if s.sourceListCacheInvalidator != nil {
		_ = s.sourceListCacheInvalidator.DeleteUser(ctx, userID)
	}
}

func (s *CollectionService) observe(ctx context.Context, observation CollectionObservation) {
	attrs := []any{
		slog.Int64("run_id", observation.RunID),
		slog.Int64("source_id", observation.SourceID),
		slog.String("source_type", observation.SourceType),
		slog.String("status", observation.Status),
		slog.Int("fetched_count", observation.FetchedCount),
		slog.Int("inserted_count", observation.InsertedCount),
		slog.Int("duplicated_count", observation.DuplicatedCount),
		slog.Duration("duration", observation.Duration),
	}
	if observation.ErrorMessage != "" {
		attrs = append(attrs, slog.String("error_message", observation.ErrorMessage))
		s.logger.Warn("collection run completed", attrs...)
	} else {
		s.logger.Info("collection run completed", attrs...)
	}

	if s.observer == nil {
		return
	}
	s.observer.ObserveCollection(ctx, observation)
}

func normalizeCollectionRunLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func normalizeCollectionRunOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

func toCollectionRunDTOs(runs []CollectionRun) []CollectionRunDTO {
	items := make([]CollectionRunDTO, 0, len(runs))
	for _, run := range runs {
		items = append(items, toCollectionRunDTO(run))
	}
	return items
}

func toCollectionRunDTO(run CollectionRun) CollectionRunDTO {
	var finishedAt *string
	if run.FinishedAt != nil {
		value := run.FinishedAt.Format(time.RFC3339Nano)
		finishedAt = &value
	}

	return CollectionRunDTO{
		ID:              run.ID,
		SourceID:        run.SourceID,
		Status:          run.Status,
		StartedAt:       run.StartedAt.Format(time.RFC3339Nano),
		FinishedAt:      finishedAt,
		FetchedCount:    run.FetchedCount,
		InsertedCount:   run.InsertedCount,
		DuplicatedCount: run.DuplicatedCount,
		ErrorMessage:    run.ErrorMessage,
	}
}
