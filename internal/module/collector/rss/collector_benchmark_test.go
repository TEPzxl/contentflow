package rss

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/tepzxl/contentflow/internal/module/source"
	"gorm.io/datatypes"
)

func BenchmarkCollector_Collect(b *testing.B) {
	feed := benchmarkRSSFeed(100)
	collector := NewCollector(WithFetcher(benchmarkFetcher{body: feed}))
	rawURL := "https://example.com/feed.xml"
	src := &source.Source{
		ID:         42,
		UserID:     100,
		Type:       source.TypeRSS,
		URL:        &rawURL,
		ConfigJSON: datatypes.JSON([]byte(`{}`)),
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		items, err := collector.Collect(context.Background(), src)
		if err != nil {
			b.Fatalf("Collect() error = %v", err)
		}
		if len(items) != 100 {
			b.Fatalf("items = %d, want 100", len(items))
		}
	}
}

type benchmarkFetcher struct {
	body string
}

func (f benchmarkFetcher) Fetch(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(f.body)), nil
}

func benchmarkRSSFeed(items int) string {
	var builder strings.Builder
	builder.WriteString(`<?xml version="1.0" encoding="UTF-8"?><rss version="2.0"><channel><title>Bench</title>`)
	for i := 0; i < items; i++ {
		builder.WriteString(fmt.Sprintf(
			`<item><title>Article %d</title><link>https://example.com/%d</link><guid>guid-%d</guid><description>Summary %d</description></item>`,
			i, i, i, i,
		))
	}
	builder.WriteString(`</channel></rss>`)
	return builder.String()
}
