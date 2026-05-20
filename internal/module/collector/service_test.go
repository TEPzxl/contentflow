package collector_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tepzxl/contentflow/internal/module/collector"
	collectormocks "github.com/tepzxl/contentflow/internal/module/collector/mocks"
	"github.com/tepzxl/contentflow/internal/module/source"
	sourcemocks "github.com/tepzxl/contentflow/internal/module/source/mocks"
	"go.uber.org/mock/gomock"
	"gorm.io/datatypes"
)

var errRepo = errors.New("repository error")
var errCollect = errors.New("collect error")
var errArticleWriter = errors.New("article writer error")

func TestCollectionService_CollectSource(t *testing.T) {
	now := fixedTime()
	src := sampleSourceModel()
	items := sampleCollectedItems()

	tests := []struct {
		name     string
		req      collector.CollectSourceRequest
		registry func(t *testing.T) *collector.Registry
		mock     func(
			t *testing.T,
			ctx context.Context,
			sourceRepo *sourcemocks.MockRepository,
			runRepo *collectormocks.MockRunRepository,
			articleWriter *collectormocks.MockArticleWriter,
		)
		wantErr        error
		wantStatus     string
		wantFetched    int
		wantInserted   int
		wantDuplicated int
	}{
		{
			name: "source not found becomes not accessible",
			req: collector.CollectSourceRequest{
				UserID:   100,
				SourceID: 999,
			},
			registry: func(t *testing.T) *collector.Registry {
				t.Helper()

				r, err := collector.NewRegistry()
				if err != nil {
					t.Fatalf("NewRegistry() error = %v", err)
				}
				return r
			},
			mock: func(
				t *testing.T,
				ctx context.Context,
				sourceRepo *sourcemocks.MockRepository,
				runRepo *collectormocks.MockRunRepository,
				articleWriter *collectormocks.MockArticleWriter,
			) {
				sourceRepo.EXPECT().
					FindByUserIDAndID(ctx, int64(100), int64(999)).
					Return(nil, source.ErrSourceNotFound)
			},
			wantErr: source.ErrSourceNotAccessible,
		},
		{
			name: "collector not found",
			req: collector.CollectSourceRequest{
				UserID:   100,
				SourceID: 1,
			},
			registry: func(t *testing.T) *collector.Registry {
				t.Helper()

				r, err := collector.NewRegistry()
				if err != nil {
					t.Fatalf("NewRegistry() error = %v", err)
				}
				return r
			},
			mock: func(
				t *testing.T,
				ctx context.Context,
				sourceRepo *sourcemocks.MockRepository,
				runRepo *collectormocks.MockRunRepository,
				articleWriter *collectormocks.MockArticleWriter,
			) {
				sourceRepo.EXPECT().
					FindByUserIDAndID(ctx, int64(100), int64(1)).
					Return(src, nil)
			},
			wantErr: collector.ErrCollectorNotFound,
		},
		{
			name: "run create failed",
			req: collector.CollectSourceRequest{
				UserID:   100,
				SourceID: 1,
			},
			registry: func(t *testing.T) *collector.Registry {
				t.Helper()

				r, err := collector.NewRegistry(fakeCollector{
					sourceType: source.TypeRSS,
					items:      items,
				})
				if err != nil {
					t.Fatalf("NewRegistry() error = %v", err)
				}
				return r
			},
			mock: func(
				t *testing.T,
				ctx context.Context,
				sourceRepo *sourcemocks.MockRepository,
				runRepo *collectormocks.MockRunRepository,
				articleWriter *collectormocks.MockArticleWriter,
			) {
				sourceRepo.EXPECT().
					FindByUserIDAndID(ctx, int64(100), int64(1)).
					Return(src, nil)

				runRepo.EXPECT().
					Create(ctx, gomock.AssignableToTypeOf(&collector.CollectionRun{})).
					Return(errRepo)
			},
			wantErr: errRepo,
		},
		{
			name: "collector collect failed",
			req: collector.CollectSourceRequest{
				UserID:   100,
				SourceID: 1,
			},
			registry: func(t *testing.T) *collector.Registry {
				t.Helper()

				r, err := collector.NewRegistry(fakeCollector{
					sourceType: source.TypeRSS,
					err:        errCollect,
				})
				if err != nil {
					t.Fatalf("NewRegistry() error = %v", err)
				}
				return r
			},
			mock: func(
				t *testing.T,
				ctx context.Context,
				sourceRepo *sourcemocks.MockRepository,
				runRepo *collectormocks.MockRunRepository,
				articleWriter *collectormocks.MockArticleWriter,
			) {
				sourceRepo.EXPECT().
					FindByUserIDAndID(ctx, int64(100), int64(1)).
					Return(src, nil)

				runRepo.EXPECT().
					Create(ctx, gomock.AssignableToTypeOf(&collector.CollectionRun{})).
					DoAndReturn(func(_ context.Context, run *collector.CollectionRun) error {
						run.ID = 10

						if run.Status != collector.RunStatusRunning {
							t.Fatalf("run.Status = %s, want running", run.Status)
						}

						if run.SourceID != 1 {
							t.Fatalf("run.SourceID = %d, want 1", run.SourceID)
						}

						return nil
					})

				runRepo.EXPECT().
					Finish(ctx, collector.FinishRunParams{
						RunID:           10,
						Status:          collector.RunStatusFailed,
						FinishedAt:      now,
						FetchedCount:    0,
						InsertedCount:   0,
						DuplicatedCount: 0,
						ErrorMessage:    errCollect.Error(),
					}).
					Return(nil)

				sourceRepo.EXPECT().
					Update(ctx, gomock.AssignableToTypeOf(&source.Source{})).
					DoAndReturn(func(_ context.Context, updated *source.Source) error {
						if updated.LastFetchStatus != collector.RunStatusFailed {
							t.Fatalf("LastFetchStatus = %s, want failed", updated.LastFetchStatus)
						}

						if updated.LastFetchMessage != errCollect.Error() {
							t.Fatalf("LastFetchMessage = %s, want %s", updated.LastFetchMessage, errCollect.Error())
						}

						if updated.LastFetchedAt == nil || !updated.LastFetchedAt.Equal(now) {
							t.Fatalf("LastFetchedAt = %v, want %v", updated.LastFetchedAt, now)
						}

						return nil
					})
			},
			wantErr:     collector.ErrCollectionFailed,
			wantStatus:  collector.RunStatusFailed,
			wantFetched: 0,
		},
		{
			name: "article writer failed",
			req: collector.CollectSourceRequest{
				UserID:   100,
				SourceID: 1,
			},
			registry: func(t *testing.T) *collector.Registry {
				t.Helper()

				r, err := collector.NewRegistry(fakeCollector{
					sourceType: source.TypeRSS,
					items:      items,
				})
				if err != nil {
					t.Fatalf("NewRegistry() error = %v", err)
				}
				return r
			},
			mock: func(
				t *testing.T,
				ctx context.Context,
				sourceRepo *sourcemocks.MockRepository,
				runRepo *collectormocks.MockRunRepository,
				articleWriter *collectormocks.MockArticleWriter,
			) {
				sourceRepo.EXPECT().
					FindByUserIDAndID(ctx, int64(100), int64(1)).
					Return(src, nil)

				runRepo.EXPECT().
					Create(ctx, gomock.AssignableToTypeOf(&collector.CollectionRun{})).
					DoAndReturn(func(_ context.Context, run *collector.CollectionRun) error {
						run.ID = 11
						return nil
					})

				articleWriter.EXPECT().
					SaveCollectedItems(ctx, items).
					Return(nil, errArticleWriter)

				runRepo.EXPECT().
					Finish(ctx, collector.FinishRunParams{
						RunID:           11,
						Status:          collector.RunStatusFailed,
						FinishedAt:      now,
						FetchedCount:    len(items),
						InsertedCount:   0,
						DuplicatedCount: 0,
						ErrorMessage:    errArticleWriter.Error(),
					}).
					Return(nil)

				sourceRepo.EXPECT().
					Update(ctx, gomock.AssignableToTypeOf(&source.Source{})).
					DoAndReturn(func(_ context.Context, updated *source.Source) error {
						if updated.LastFetchStatus != collector.RunStatusFailed {
							t.Fatalf("LastFetchStatus = %s, want failed", updated.LastFetchStatus)
						}

						if updated.LastFetchMessage != errArticleWriter.Error() {
							t.Fatalf("LastFetchMessage = %s, want %s", updated.LastFetchMessage, errArticleWriter.Error())
						}

						return nil
					})
			},
			wantErr:     collector.ErrCollectionFailed,
			wantStatus:  collector.RunStatusFailed,
			wantFetched: len(items),
		},
		{
			name: "success",
			req: collector.CollectSourceRequest{
				UserID:   100,
				SourceID: 1,
			},
			registry: func(t *testing.T) *collector.Registry {
				t.Helper()

				r, err := collector.NewRegistry(fakeCollector{
					sourceType: source.TypeRSS,
					items:      items,
				})
				if err != nil {
					t.Fatalf("NewRegistry() error = %v", err)
				}
				return r
			},
			mock: func(
				t *testing.T,
				ctx context.Context,
				sourceRepo *sourcemocks.MockRepository,
				runRepo *collectormocks.MockRunRepository,
				articleWriter *collectormocks.MockArticleWriter,
			) {
				sourceRepo.EXPECT().
					FindByUserIDAndID(ctx, int64(100), int64(1)).
					Return(src, nil)

				runRepo.EXPECT().
					Create(ctx, gomock.AssignableToTypeOf(&collector.CollectionRun{})).
					DoAndReturn(func(_ context.Context, run *collector.CollectionRun) error {
						run.ID = 12

						if run.Status != collector.RunStatusRunning {
							t.Fatalf("run.Status = %s, want running", run.Status)
						}

						return nil
					})

				articleWriter.EXPECT().
					SaveCollectedItems(ctx, items).
					Return(&collector.ArticleWriteResult{
						InsertedCount:   1,
						DuplicatedCount: 1,
					}, nil)

				runRepo.EXPECT().
					Finish(ctx, collector.FinishRunParams{
						RunID:           12,
						Status:          collector.RunStatusSuccess,
						FinishedAt:      now,
						FetchedCount:    len(items),
						InsertedCount:   1,
						DuplicatedCount: 1,
						ErrorMessage:    "",
					}).
					Return(nil)

				sourceRepo.EXPECT().
					Update(ctx, gomock.AssignableToTypeOf(&source.Source{})).
					DoAndReturn(func(_ context.Context, updated *source.Source) error {
						if updated.LastFetchStatus != collector.RunStatusSuccess {
							t.Fatalf("LastFetchStatus = %s, want success", updated.LastFetchStatus)
						}

						if updated.LastFetchMessage != "" {
							t.Fatalf("LastFetchMessage = %s, want empty", updated.LastFetchMessage)
						}

						if updated.LastFetchedAt == nil || !updated.LastFetchedAt.Equal(now) {
							t.Fatalf("LastFetchedAt = %v, want %v", updated.LastFetchedAt, now)
						}

						return nil
					})
			},
			wantErr:        nil,
			wantStatus:     collector.RunStatusSuccess,
			wantFetched:    len(items),
			wantInserted:   1,
			wantDuplicated: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ctx := context.Background()

			sourceRepo := sourcemocks.NewMockRepository(ctrl)
			runRepo := collectormocks.NewMockRunRepository(ctrl)
			articleWriter := collectormocks.NewMockArticleWriter(ctrl)

			registry := tt.registry(t)
			tt.mock(t, ctx, sourceRepo, runRepo, articleWriter)

			svc := collector.NewService(
				sourceRepo,
				runRepo,
				registry,
				articleWriter,
				collector.WithNow(func() time.Time {
					return now
				}),
			)

			resp, err := svc.CollectSource(ctx, tt.req)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("CollectSource() error = %v, want %v", err, tt.wantErr)
				}

				if tt.wantStatus != "" {
					if resp == nil {
						t.Fatal("CollectSource() response is nil")
					}

					if resp.Status != tt.wantStatus {
						t.Fatalf("response status = %s, want %s", resp.Status, tt.wantStatus)
					}

					if resp.FetchedCount != tt.wantFetched {
						t.Fatalf("fetched count = %d, want %d", resp.FetchedCount, tt.wantFetched)
					}
				}

				return
			}

			if err != nil {
				t.Fatalf("CollectSource() unexpected error = %v", err)
			}

			if resp == nil {
				t.Fatal("CollectSource() response is nil")
			}

			if resp.Status != tt.wantStatus {
				t.Fatalf("response status = %s, want %s", resp.Status, tt.wantStatus)
			}

			if resp.FetchedCount != tt.wantFetched {
				t.Fatalf("fetched count = %d, want %d", resp.FetchedCount, tt.wantFetched)
			}

			if resp.InsertedCount != tt.wantInserted {
				t.Fatalf("inserted count = %d, want %d", resp.InsertedCount, tt.wantInserted)
			}

			if resp.DuplicatedCount != tt.wantDuplicated {
				t.Fatalf("duplicated count = %d, want %d", resp.DuplicatedCount, tt.wantDuplicated)
			}
		})
	}
}

func TestCollectionService_CollectSource_returnsInProgressWhenLockNotAcquired(t *testing.T) {
	now := fixedTime()
	ctx := context.Background()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sourceRepo := sourcemocks.NewMockRepository(ctrl)
	runRepo := collectormocks.NewMockRunRepository(ctrl)
	articleWriter := collectormocks.NewMockArticleWriter(ctrl)

	sourceRepo.EXPECT().
		FindByUserIDAndID(ctx, int64(100), int64(1)).
		Return(sampleSourceModel(), nil)

	registry, err := collector.NewRegistry(fakeCollector{sourceType: source.TypeRSS})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	svc := collector.NewService(
		sourceRepo,
		runRepo,
		registry,
		articleWriter,
		collector.WithNow(func() time.Time { return now }),
		collector.WithCollectionLock(&fakeCollectionLock{acquired: false}),
	)

	resp, err := svc.CollectSource(ctx, collector.CollectSourceRequest{
		UserID:   100,
		SourceID: 1,
	})
	if !errors.Is(err, collector.ErrCollectionInProgress) {
		t.Fatalf("CollectSource() error = %v, want %v", err, collector.ErrCollectionInProgress)
	}
	if resp != nil {
		t.Fatalf("CollectSource() response = %#v, want nil", resp)
	}
}

func TestCollectionService_ListCollectionRuns(t *testing.T) {
	ctx := context.Background()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sourceRepo := sourcemocks.NewMockRepository(ctrl)
	runRepo := collectormocks.NewMockRunRepository(ctrl)
	articleWriter := collectormocks.NewMockArticleWriter(ctrl)

	sourceRepo.EXPECT().
		FindByUserIDAndID(ctx, int64(100), int64(1)).
		Return(sampleSourceModel(), nil)

	runRepo.EXPECT().
		ListBySourceID(ctx, collector.ListRunsParams{
			SourceID: 1,
			Status:   collector.RunStatusFailed,
			Limit:    100,
			Offset:   0,
		}).
		Return([]collector.CollectionRun{
			sampleCollectionRun(9, collector.RunStatusFailed),
		}, int64(1), nil)

	registry, err := collector.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	svc := collector.NewService(sourceRepo, runRepo, registry, articleWriter)
	resp, err := svc.ListCollectionRuns(ctx, collector.ListCollectionRunsRequest{
		UserID:   100,
		SourceID: 1,
		Status:   collector.RunStatusFailed,
		Limit:    200,
		Offset:   -1,
	})
	if err != nil {
		t.Fatalf("ListCollectionRuns() error = %v", err)
	}
	if resp.Total != 1 {
		t.Fatalf("Total = %d, want 1", resp.Total)
	}
	if resp.Limit != 100 {
		t.Fatalf("Limit = %d, want 100", resp.Limit)
	}
	if resp.Offset != 0 {
		t.Fatalf("Offset = %d, want 0", resp.Offset)
	}
	if len(resp.Runs) != 1 || resp.Runs[0].ID != 9 {
		t.Fatalf("Runs = %#v, want run id 9", resp.Runs)
	}
}

func TestCollectionService_ListCollectionRuns_sourceNotAccessible(t *testing.T) {
	ctx := context.Background()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sourceRepo := sourcemocks.NewMockRepository(ctrl)
	runRepo := collectormocks.NewMockRunRepository(ctrl)
	articleWriter := collectormocks.NewMockArticleWriter(ctrl)

	sourceRepo.EXPECT().
		FindByUserIDAndID(ctx, int64(100), int64(999)).
		Return(nil, source.ErrSourceNotFound)

	registry, err := collector.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	svc := collector.NewService(sourceRepo, runRepo, registry, articleWriter)
	resp, err := svc.ListCollectionRuns(ctx, collector.ListCollectionRunsRequest{
		UserID:   100,
		SourceID: 999,
	})
	if !errors.Is(err, source.ErrSourceNotAccessible) {
		t.Fatalf("ListCollectionRuns() error = %v, want %v", err, source.ErrSourceNotAccessible)
	}
	if resp != nil {
		t.Fatalf("ListCollectionRuns() response = %#v, want nil", resp)
	}
}

func TestCollectionService_GetCollectionRun(t *testing.T) {
	ctx := context.Background()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sourceRepo := sourcemocks.NewMockRepository(ctrl)
	runRepo := collectormocks.NewMockRunRepository(ctrl)
	articleWriter := collectormocks.NewMockArticleWriter(ctrl)

	runRepo.EXPECT().
		FindByUserIDAndID(ctx, int64(100), int64(9)).
		Return(ptrCollectionRun(sampleCollectionRun(9, collector.RunStatusSuccess)), nil)

	registry, err := collector.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	svc := collector.NewService(sourceRepo, runRepo, registry, articleWriter)
	resp, err := svc.GetCollectionRun(ctx, collector.GetCollectionRunRequest{
		UserID: 100,
		RunID:  9,
	})
	if err != nil {
		t.Fatalf("GetCollectionRun() error = %v", err)
	}
	if resp.Run.ID != 9 {
		t.Fatalf("Run.ID = %d, want 9", resp.Run.ID)
	}
	if resp.Run.StartedAt == "" {
		t.Fatal("Run.StartedAt is empty")
	}
	if resp.Run.FinishedAt == nil || *resp.Run.FinishedAt == "" {
		t.Fatalf("Run.FinishedAt = %v, want non-empty", resp.Run.FinishedAt)
	}
}

type fakeCollectionLock struct {
	acquired bool
	released bool
}

func (l *fakeCollectionLock) Acquire(ctx context.Context, sourceID int64, ttl time.Duration) (collector.CollectionLockReleaseFunc, bool, error) {
	if !l.acquired {
		return nil, false, nil
	}
	return func(context.Context) error {
		l.released = true
		return nil
	}, true, nil
}

func fixedTime() time.Time {
	return time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
}

func sampleSourceModel() *source.Source {
	now := fixedTime()
	rawURL := "https://go.dev/blog/feed.atom"

	return &source.Source{
		ID:               1,
		UserID:           100,
		Name:             "Go Blog",
		Type:             source.TypeRSS,
		URL:              &rawURL,
		ConfigJSON:       datatypes.JSON([]byte(`{}`)),
		IsActive:         true,
		LastFetchStatus:  "",
		LastFetchMessage: "",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

func sampleCollectionRun(id int64, status string) collector.CollectionRun {
	now := fixedTime()

	return collector.CollectionRun{
		ID:              id,
		SourceID:        1,
		Status:          status,
		StartedAt:       now.Add(-time.Minute),
		FinishedAt:      &now,
		FetchedCount:    3,
		InsertedCount:   2,
		DuplicatedCount: 1,
		ErrorMessage:    "",
		CreatedAt:       now.Add(-time.Minute),
	}
}

func ptrCollectionRun(run collector.CollectionRun) *collector.CollectionRun {
	return &run
}

func sampleCollectedItems() []collector.CollectedItem {
	externalID1 := "rss-guid-1"
	externalID2 := "rss-guid-2"
	url1 := "https://go.dev/blog/1"
	url2 := "https://go.dev/blog/2"
	now := fixedTime()

	return []collector.CollectedItem{
		{
			SourceID:    1,
			SourceType:  source.TypeRSS,
			ExternalID:  &externalID1,
			Title:       "Go Blog 1",
			URL:         &url1,
			Content:     "content 1",
			ContentHash: "hash-1",
			PublishedAt: &now,
		},
		{
			SourceID:    1,
			SourceType:  source.TypeRSS,
			ExternalID:  &externalID2,
			Title:       "Go Blog 2",
			URL:         &url2,
			Content:     "content 2",
			ContentHash: "hash-2",
			PublishedAt: &now,
		},
	}
}
