package collector

import (
	"context"

	"github.com/tepzxl/contentflow/internal/module/source"
)

type Collector interface {
	Type() string
	Collect(ctx context.Context, src *source.Source) ([]CollectedItem, error)
}
