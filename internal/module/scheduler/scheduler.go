package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/tepzxl/contentflow/internal/module/collector"
	"github.com/tepzxl/contentflow/internal/module/source"
)

const (
	defaultInterval    = 15 * time.Minute
	defaultBatchSize   = 100
	defaultConcurrency = 4
)

type SourceLister interface {
	ListActiveForCollection(ctx context.Context, limit int) ([]source.ActiveSourceForCollection, error)
}

type CollectionService interface {
	CollectSource(ctx context.Context, req collector.CollectSourceRequest) (*collector.CollectSourceResponse, error)
}

type Scheduler struct {
	sourceLister SourceLister
	collector    CollectionService
	logger       *slog.Logger
	interval     time.Duration
	batchSize    int
	concurrency  int
}

type Option func(*Scheduler)

func WithInterval(interval time.Duration) Option {
	return func(s *Scheduler) {
		if interval > 0 {
			s.interval = interval
		}
	}
}

func WithBatchSize(batchSize int) Option {
	return func(s *Scheduler) {
		if batchSize > 0 {
			s.batchSize = batchSize
		}
	}
}

func WithConcurrency(concurrency int) Option {
	return func(s *Scheduler) {
		if concurrency > 0 {
			s.concurrency = concurrency
		}
	}
}

func WithLogger(logger *slog.Logger) Option {
	return func(s *Scheduler) {
		if logger != nil {
			s.logger = logger
		}
	}
}

func New(sourceLister SourceLister, collector CollectionService, opts ...Option) *Scheduler {
	s := &Scheduler{
		sourceLister: sourceLister,
		collector:    collector,
		logger:       slog.Default(),
		interval:     defaultInterval,
		batchSize:    defaultBatchSize,
		concurrency:  defaultConcurrency,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *Scheduler) Run(ctx context.Context) error {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := s.RunOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
				s.logger.Error("scheduled collection failed", slog.String("error", err.Error()))
			}
		}
	}
}

func (s *Scheduler) RunOnce(ctx context.Context) error {
	sources, err := s.sourceLister.ListActiveForCollection(ctx, s.batchSize)
	if err != nil {
		return fmt.Errorf("list active sources: %w", err)
	}

	if len(sources) == 0 {
		return nil
	}

	workers := s.concurrency
	if workers <= 0 {
		workers = 1
	}
	if workers > len(sources) {
		workers = len(sources)
	}

	jobs := make(chan source.ActiveSourceForCollection)
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for src := range jobs {
				s.collectOne(ctx, src)
			}
		}()
	}

	for _, src := range sources {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return ctx.Err()
		case jobs <- src:
		}
	}

	close(jobs)
	wg.Wait()

	return nil
}

func (s *Scheduler) collectOne(ctx context.Context, src source.ActiveSourceForCollection) {
	resp, err := s.collector.CollectSource(ctx, collector.CollectSourceRequest{
		UserID:   src.UserID,
		SourceID: src.ID,
	})
	if err != nil {
		s.logger.Warn(
			"scheduled source collection failed",
			slog.Int64("source_id", src.ID),
			slog.Int64("user_id", src.UserID),
			slog.String("source_type", src.Type),
			slog.String("error", err.Error()),
		)
		return
	}

	if resp != nil {
		s.logger.Info(
			"scheduled source collection finished",
			slog.Int64("source_id", src.ID),
			slog.Int64("user_id", src.UserID),
			slog.String("source_type", src.Type),
			slog.String("status", resp.Status),
			slog.Int("fetched_count", resp.FetchedCount),
			slog.Int("inserted_count", resp.InsertedCount),
			slog.Int("duplicated_count", resp.DuplicatedCount),
		)
	}
}
