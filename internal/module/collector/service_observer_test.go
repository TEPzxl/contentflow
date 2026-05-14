package collector_test

import (
	"context"
	"testing"
	"time"

	"github.com/tepzxl/contentflow/internal/module/collector"
	collectormocks "github.com/tepzxl/contentflow/internal/module/collector/mocks"
	"github.com/tepzxl/contentflow/internal/module/source"
	sourcemocks "github.com/tepzxl/contentflow/internal/module/source/mocks"
	"go.uber.org/mock/gomock"
)

type recordingCollectionObserver struct {
	observations []collector.CollectionObservation
}

func (o *recordingCollectionObserver) ObserveCollection(_ context.Context, observation collector.CollectionObservation) {
	o.observations = append(o.observations, observation)
}

func TestCollectionService_ObserverReceivesSuccessfulCollection(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	now := fixedTime()
	src := sampleSourceModel()
	items := sampleCollectedItems()

	sourceRepo := sourcemocks.NewMockRepository(ctrl)
	runRepo := collectormocks.NewMockRunRepository(ctrl)
	articleWriter := collectormocks.NewMockArticleWriter(ctrl)

	sourceRepo.EXPECT().
		FindByUserIDAndID(ctx, int64(100), int64(1)).
		Return(src, nil)
	runRepo.EXPECT().
		Create(ctx, gomock.AssignableToTypeOf(&collector.CollectionRun{})).
		DoAndReturn(func(_ context.Context, run *collector.CollectionRun) error {
			run.ID = 20
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
			RunID:           20,
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
		Return(nil)

	registry, err := collector.NewRegistry(fakeCollector{
		sourceType: source.TypeRSS,
		items:      items,
	})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	observer := &recordingCollectionObserver{}
	svc := collector.NewService(
		sourceRepo,
		runRepo,
		registry,
		articleWriter,
		collector.WithNow(func() time.Time { return now }),
		collector.WithObserver(observer),
	)

	_, err = svc.CollectSource(ctx, collector.CollectSourceRequest{
		UserID:   100,
		SourceID: 1,
	})
	if err != nil {
		t.Fatalf("CollectSource() error = %v", err)
	}

	if len(observer.observations) != 1 {
		t.Fatalf("observer calls = %d, want 1", len(observer.observations))
	}

	got := observer.observations[0]
	if got.RunID != 20 {
		t.Fatalf("RunID = %d, want 20", got.RunID)
	}
	if got.SourceType != source.TypeRSS {
		t.Fatalf("SourceType = %s, want %s", got.SourceType, source.TypeRSS)
	}
	if got.Status != collector.RunStatusSuccess {
		t.Fatalf("Status = %s, want %s", got.Status, collector.RunStatusSuccess)
	}
	if got.FetchedCount != len(items) || got.InsertedCount != 1 || got.DuplicatedCount != 1 {
		t.Fatalf("counts = fetched:%d inserted:%d duplicated:%d", got.FetchedCount, got.InsertedCount, got.DuplicatedCount)
	}
}
