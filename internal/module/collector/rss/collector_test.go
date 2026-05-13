package rss

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/tepzxl/contentflow/internal/module/collector"
	"github.com/tepzxl/contentflow/internal/module/source"
	"gorm.io/datatypes"
)

var (
	errFetch = errors.New("fetch failed")
	errParse = errors.New("parse failed")
)

func TestCollector_Collect(t *testing.T) {
	publishedAt := time.Date(2026, 5, 13, 10, 0, 0, 0, time.UTC)
	feedURL := "https://example.com/feed.xml"

	tests := []struct {
		name    string
		src     *source.Source
		fetcher *fakeFetcher
		parser  *fakeParser
		want    func(t *testing.T, items []collector.CollectedItem)
		wantErr error
	}{
		{
			name: "source URL required",
			src: &source.Source{
				ID:   1,
				Type: source.TypeRSS,
			},
			fetcher: &fakeFetcher{},
			parser:  &fakeParser{},
			wantErr: ErrSourceURLRequired,
		},
		{
			name: "fetcher failure is wrapped",
			src:  sampleRSSSource(feedURL),
			fetcher: &fakeFetcher{
				err: errFetch,
			},
			parser:  &fakeParser{},
			wantErr: ErrFeedFetchFailed,
		},
		{
			name: "parser failure is wrapped and body closed",
			src:  sampleRSSSource(feedURL),
			fetcher: &fakeFetcher{
				body: newTrackingReadCloser("<rss></rss>"),
			},
			parser: &fakeParser{
				err: errParse,
			},
			wantErr: ErrFeedParseFailed,
			want: func(t *testing.T, _ []collector.CollectedItem) {
				t.Helper()
			},
		},
		{
			name: "item fields are converted and empty title is skipped",
			src:  sampleRSSSource(feedURL),
			fetcher: &fakeFetcher{
				body: newTrackingReadCloser("<rss></rss>"),
			},
			parser: &fakeParser{
				feed: &gofeed.Feed{
					Items: []*gofeed.Item{
						{
							GUID:            " guid-1 ",
							Title:           " First item ",
							Link:            " https://example.com/articles/1 ",
							Author:          &gofeed.Person{Name: " Ada "},
							Description:     " Summary text ",
							Content:         " Full content ",
							PublishedParsed: &publishedAt,
						},
						{
							Title: "   ",
							Link:  "https://example.com/articles/empty-title",
						},
					},
				},
			},
			want: func(t *testing.T, items []collector.CollectedItem) {
				t.Helper()

				if len(items) != 1 {
					t.Fatalf("len(items) = %d, want 1", len(items))
				}

				item := items[0]
				if item.SourceID != 10 {
					t.Fatalf("SourceID = %d, want 10", item.SourceID)
				}
				if item.SourceType != source.TypeRSS {
					t.Fatalf("SourceType = %q, want %q", item.SourceType, source.TypeRSS)
				}
				assertStringPtr(t, "ExternalID", item.ExternalID, "guid-1")
				if item.Title != "First item" {
					t.Fatalf("Title = %q, want %q", item.Title, "First item")
				}
				assertStringPtr(t, "URL", item.URL, "https://example.com/articles/1")
				assertStringPtr(t, "OriginalURL", item.OriginalURL, "https://example.com/articles/1")
				if item.Author != "Ada" {
					t.Fatalf("Author = %q, want %q", item.Author, "Ada")
				}
				if item.Summary != "Summary text" {
					t.Fatalf("Summary = %q, want %q", item.Summary, "Summary text")
				}
				if item.Content != "Full content" {
					t.Fatalf("Content = %q, want %q", item.Content, "Full content")
				}
				if item.ContentHash == "" || len(item.ContentHash) != 64 {
					t.Fatalf("ContentHash = %q, want 64-char hash", item.ContentHash)
				}
				if item.PublishedAt == nil || !item.PublishedAt.Equal(publishedAt) {
					t.Fatalf("PublishedAt = %v, want %v", item.PublishedAt, publishedAt)
				}
			},
		},
		{
			name: "content hash is stable for same normalized content",
			src:  sampleRSSSource(feedURL),
			fetcher: &fakeFetcher{
				body: newTrackingReadCloser("<rss></rss>"),
			},
			parser: &fakeParser{
				feed: &gofeed.Feed{
					Items: []*gofeed.Item{
						{
							GUID:    "guid-1",
							Title:   "First item",
							Link:    "https://example.com/articles/1",
							Content: "Full   content\nwith\tspaces",
						},
						{
							GUID:    "guid-1",
							Title:   " First item ",
							Link:    " https://example.com/articles/1 ",
							Content: "Full content with spaces",
						},
					},
				},
			},
			want: func(t *testing.T, items []collector.CollectedItem) {
				t.Helper()

				if len(items) != 2 {
					t.Fatalf("len(items) = %d, want 2", len(items))
				}
				if items[0].ContentHash != items[1].ContentHash {
					t.Fatalf("ContentHash mismatch: %q != %q", items[0].ContentHash, items[1].ContentHash)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewCollector(
				WithFetcher(tt.fetcher),
				WithParser(tt.parser),
			)

			items, err := c.Collect(context.Background(), tt.src)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Collect() error = %v, want %v", err, tt.wantErr)
				}
				if tt.fetcher != nil && tt.fetcher.body != nil && !tt.fetcher.body.closed {
					t.Fatal("Collect() did not close fetched body after error")
				}
				if errors.Is(tt.wantErr, ErrFeedFetchFailed) && tt.parser.called {
					t.Fatal("Collect() called parser after fetcher failure")
				}
				return
			}

			if err != nil {
				t.Fatalf("Collect() unexpected error = %v", err)
			}
			if tt.want != nil {
				tt.want(t, items)
			}
			if tt.fetcher != nil && tt.fetcher.body != nil && !tt.fetcher.body.closed {
				t.Fatal("Collect() did not close fetched body")
			}
		})
	}
}

func TestHTTPFetcher_Fetch(t *testing.T) {
	tests := []struct {
		name       string
		ctx        func() context.Context
		statusCode int
		body       *trackingReadCloser
		wantErr    bool
		wantClosed bool
	}{
		{
			name:       "success returns response body",
			ctx:        context.Background,
			statusCode: http.StatusOK,
			body:       newTrackingReadCloser("ok"),
			wantErr:    false,
			wantClosed: false,
		},
		{
			name:       "non 2xx status closes response body",
			ctx:        context.Background,
			statusCode: http.StatusInternalServerError,
			body:       newTrackingReadCloser("failed"),
			wantErr:    true,
			wantClosed: true,
		},
		{
			name: "context cancellation is propagated",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			statusCode: http.StatusOK,
			body:       newTrackingReadCloser("ok"),
			wantErr:    true,
			wantClosed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					if err := req.Context().Err(); err != nil {
						return nil, err
					}

					if got := req.Header.Get("User-Agent"); got == "" {
						t.Fatal("User-Agent header is empty")
					}

					return &http.Response{
						StatusCode: tt.statusCode,
						Body:       tt.body,
						Header:     make(http.Header),
						Request:    req,
					}, nil
				}),
			}
			fetcher := NewHTTPFetcher(client)

			body, err := fetcher.Fetch(tt.ctx(), "https://example.com/feed.xml")

			if tt.wantErr {
				if err == nil {
					t.Fatal("Fetch() expected error, got nil")
				}
				if body != nil {
					t.Fatal("Fetch() body must be nil on error")
				}
			} else {
				if err != nil {
					t.Fatalf("Fetch() unexpected error = %v", err)
				}
				if body == nil {
					t.Fatal("Fetch() body is nil")
				}
			}

			if tt.body.closed != tt.wantClosed {
				t.Fatalf("body.closed = %v, want %v", tt.body.closed, tt.wantClosed)
			}

			if body != nil {
				_ = body.Close()
			}
		})
	}
}

type fakeFetcher struct {
	body *trackingReadCloser
	err  error
}

func (f *fakeFetcher) Fetch(ctx context.Context, feedURL string) (io.ReadCloser, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.body, nil
}

type fakeParser struct {
	feed   *gofeed.Feed
	err    error
	called bool
}

func (p *fakeParser) Parse(reader io.Reader) (*gofeed.Feed, error) {
	p.called = true
	if p.err != nil {
		return nil, p.err
	}
	if p.feed == nil {
		return &gofeed.Feed{}, nil
	}
	return p.feed, nil
}

type trackingReadCloser struct {
	*strings.Reader
	closed bool
}

func newTrackingReadCloser(value string) *trackingReadCloser {
	return &trackingReadCloser{
		Reader: strings.NewReader(value),
	}
}

func (r *trackingReadCloser) Close() error {
	r.closed = true
	return nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func sampleRSSSource(feedURL string) *source.Source {
	return &source.Source{
		ID:         10,
		UserID:     100,
		Name:       "Example RSS",
		Type:       source.TypeRSS,
		URL:        &feedURL,
		ConfigJSON: datatypes.JSON([]byte(`{}`)),
		IsActive:   true,
	}
}

func assertStringPtr(t *testing.T, field string, got *string, want string) {
	t.Helper()

	if got == nil {
		t.Fatalf("%s = nil, want %q", field, want)
	}
	if *got != want {
		t.Fatalf("%s = %q, want %q", field, *got, want)
	}
}
