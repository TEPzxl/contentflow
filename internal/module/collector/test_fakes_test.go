package collector_test

import (
	"context"

	"github.com/tepzxl/contentflow/internal/module/collector"
	"github.com/tepzxl/contentflow/internal/module/source"
)

type fakeCollector struct {
	sourceType string
	items      []collector.CollectedItem
	err        error
}

func (f fakeCollector) Type() string {
	return f.sourceType
}

func (f fakeCollector) Collect(ctx context.Context, src *source.Source) ([]collector.CollectedItem, error) {
	return f.items, f.err
}
