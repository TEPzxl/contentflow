package collector

import (
	"context"
	"errors"
	"fmt"

	"github.com/tepzxl/contentflow/internal/module/source"
)

type SourceValidatingRequester struct {
	sources  source.Repository
	delegate CollectionRequester
}

func NewSourceValidatingRequester(sources source.Repository, delegate CollectionRequester) *SourceValidatingRequester {
	return &SourceValidatingRequester{
		sources:  sources,
		delegate: delegate,
	}
}

func (r *SourceValidatingRequester) RequestCollection(ctx context.Context, req CollectSourceRequest) (*RequestCollectionResponse, error) {
	if r.sources == nil {
		return nil, fmt.Errorf("source validating requester: source repository is required")
	}
	if r.delegate == nil {
		return nil, fmt.Errorf("source validating requester: delegate is required")
	}

	if _, err := r.sources.FindByUserIDAndID(ctx, req.UserID, req.SourceID); err != nil {
		if errors.Is(err, source.ErrSourceNotFound) {
			return nil, source.ErrSourceNotAccessible
		}
		return nil, fmt.Errorf("validate source before queueing collection: %w", err)
	}

	return r.delegate.RequestCollection(ctx, req)
}
