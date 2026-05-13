package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/tepzxl/contentflow/internal/module/collector"
	"github.com/tepzxl/contentflow/internal/module/source"
)

var errListActive = errors.New("list active error")
var errCollect = errors.New("collect error")

func TestScheduler_RunOnce(t *testing.T) {
	tests := []struct {
		name    string
		lister  *fakeSourceLister
		service *fakeCollectionService
		options []Option
		wantErr error
		want    func(t *testing.T, service *fakeCollectionService)
	}{
		{
			name: "collects active sources with concurrency limit",
			lister: &fakeSourceLister{
				sources: []source.ActiveSourceForCollection{
					{ID: 1, UserID: 100, Type: source.TypeRSS},
					{ID: 2, UserID: 200, Type: source.TypeEmail},
					{ID: 3, UserID: 100, Type: source.TypeRSS},
				},
			},
			service: &fakeCollectionService{},
			options: []Option{
				WithBatchSize(10),
				WithConcurrency(2),
			},
			want: func(t *testing.T, service *fakeCollectionService) {
				t.Helper()

				if len(service.requests) != 3 {
					t.Fatalf("request count = %d, want 3", len(service.requests))
				}
				if service.maxActive > 2 {
					t.Fatalf("max active workers = %d, want <= 2", service.maxActive)
				}

				got := map[int64]int64{}
				for _, req := range service.requests {
					got[req.SourceID] = req.UserID
				}
				if got[1] != 100 || got[2] != 200 || got[3] != 100 {
					t.Fatalf("requests = %#v", service.requests)
				}
			},
		},
		{
			name: "continues after collection error",
			lister: &fakeSourceLister{
				sources: []source.ActiveSourceForCollection{
					{ID: 1, UserID: 100, Type: source.TypeRSS},
					{ID: 2, UserID: 100, Type: source.TypeRSS},
				},
			},
			service: &fakeCollectionService{
				errBySourceID: map[int64]error{
					1: errCollect,
				},
			},
			options: []Option{
				WithConcurrency(1),
			},
			want: func(t *testing.T, service *fakeCollectionService) {
				t.Helper()

				if len(service.requests) != 2 {
					t.Fatalf("request count = %d, want 2", len(service.requests))
				}
			},
		},
		{
			name: "returns list error",
			lister: &fakeSourceLister{
				err: errListActive,
			},
			service: &fakeCollectionService{},
			wantErr: errListActive,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := append([]Option{WithLogger(slog.Default())}, tt.options...)
			s := New(tt.lister, tt.service, options...)

			err := s.RunOnce(context.Background())

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("RunOnce() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("RunOnce() unexpected error = %v", err)
			}

			if tt.want != nil {
				tt.want(t, tt.service)
			}
		})
	}
}

func TestScheduler_RunStopsWhenContextCancelled(t *testing.T) {
	lister := &fakeSourceLister{}
	service := &fakeCollectionService{}

	s := New(
		lister,
		service,
		WithInterval(time.Hour),
		WithLogger(slog.Default()),
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := s.Run(ctx)
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
}

type fakeSourceLister struct {
	sources []source.ActiveSourceForCollection
	err     error
}

func (f *fakeSourceLister) ListActiveForCollection(ctx context.Context, limit int) ([]source.ActiveSourceForCollection, error) {
	if f.err != nil {
		return nil, f.err
	}

	if limit > 0 && len(f.sources) > limit {
		return f.sources[:limit], nil
	}

	return f.sources, nil
}

type fakeCollectionService struct {
	mu            sync.Mutex
	requests      []collector.CollectSourceRequest
	active        int
	maxActive     int
	errBySourceID map[int64]error
}

func (f *fakeCollectionService) CollectSource(ctx context.Context, req collector.CollectSourceRequest) (*collector.CollectSourceResponse, error) {
	f.mu.Lock()
	f.requests = append(f.requests, req)
	f.active++
	if f.active > f.maxActive {
		f.maxActive = f.active
	}
	f.mu.Unlock()

	time.Sleep(5 * time.Millisecond)

	f.mu.Lock()
	f.active--
	f.mu.Unlock()

	if err := f.errBySourceID[req.SourceID]; err != nil {
		return &collector.CollectSourceResponse{
			SourceID:     req.SourceID,
			Status:       collector.RunStatusFailed,
			ErrorMessage: err.Error(),
		}, err
	}

	return &collector.CollectSourceResponse{
		SourceID: req.SourceID,
		Status:   collector.RunStatusSuccess,
	}, nil
}
