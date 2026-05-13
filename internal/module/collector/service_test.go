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
