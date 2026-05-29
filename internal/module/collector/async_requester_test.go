package collector_test

import (
	"context"
	"errors"
	"testing"

	"github.com/tepzxl/contentflow/internal/module/collector"
	"github.com/tepzxl/contentflow/internal/module/source"
	sourcemocks "github.com/tepzxl/contentflow/internal/module/source/mocks"
	"go.uber.org/mock/gomock"
)

func TestSourceValidatingRequesterRejectsInaccessibleSourceBeforeEnqueue(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	sourceRepo := sourcemocks.NewMockRepository(ctrl)
	delegate := &fakeCollectionRequester{}
	requester := collector.NewSourceValidatingRequester(sourceRepo, delegate)

	sourceRepo.EXPECT().
		FindByUserIDAndID(ctx, int64(100), int64(42)).
		Return(nil, source.ErrSourceNotFound)

	_, err := requester.RequestCollection(ctx, collector.CollectSourceRequest{UserID: 100, SourceID: 42})
	if !errors.Is(err, source.ErrSourceNotAccessible) {
		t.Fatalf("RequestCollection() error = %v, want ErrSourceNotAccessible", err)
	}
	if len(delegate.reqs) != 0 {
		t.Fatalf("delegate request count = %d, want 0", len(delegate.reqs))
	}
}

func TestSourceValidatingRequesterDelegatesAccessibleSource(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	sourceRepo := sourcemocks.NewMockRepository(ctrl)
	delegate := &fakeCollectionRequester{resp: &collector.RequestCollectionResponse{TaskID: "task-1", SourceID: 42, Status: collector.RunStatusQueued}}
	requester := collector.NewSourceValidatingRequester(sourceRepo, delegate)

	sourceRepo.EXPECT().
		FindByUserIDAndID(ctx, int64(100), int64(42)).
		Return(&source.Source{ID: 42, UserID: 100}, nil)

	resp, err := requester.RequestCollection(ctx, collector.CollectSourceRequest{UserID: 100, SourceID: 42})
	if err != nil {
		t.Fatalf("RequestCollection() error = %v", err)
	}
	if resp == nil || resp.TaskID != "task-1" {
		t.Fatalf("response = %#v", resp)
	}
	if len(delegate.reqs) != 1 {
		t.Fatalf("delegate request count = %d, want 1", len(delegate.reqs))
	}
}
