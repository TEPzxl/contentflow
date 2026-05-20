package ai

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

type SummaryProcessor interface {
	ProcessNextSummary(ctx context.Context) (bool, error)
}

type SummaryWorker struct {
	processor SummaryProcessor
	interval  time.Duration
	logger    *slog.Logger
}

type WorkerOption func(*SummaryWorker)

func WithWorkerInterval(interval time.Duration) WorkerOption {
	return func(w *SummaryWorker) {
		if interval > 0 {
			w.interval = interval
		}
	}
}

func WithWorkerLogger(logger *slog.Logger) WorkerOption {
	return func(w *SummaryWorker) {
		if logger != nil {
			w.logger = logger
		}
	}
}

func NewSummaryWorker(processor SummaryProcessor, opts ...WorkerOption) *SummaryWorker {
	w := &SummaryWorker{
		processor: processor,
		interval:  10 * time.Second,
		logger:    slog.Default(),
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

func (w *SummaryWorker) Run(ctx context.Context) error {
	if w.processor == nil {
		return errors.New("summary processor is required")
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		processed, err := w.processor.ProcessNextSummary(ctx)
		if err != nil {
			w.logger.Warn("process ai summary failed", slog.String("error", err.Error()))
		}
		if processed {
			continue
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
