package rss

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/tepzxl/contentflow/internal/module/collector"
	"github.com/tepzxl/contentflow/internal/module/source"
	"github.com/tepzxl/contentflow/internal/netguard"
)

var (
	ErrSourceURLRequired = errors.New("source url required")
	ErrFeedParseFailed   = errors.New("feed parse failed")
	ErrFeedFetchFailed   = errors.New("feed fetch failed")
)

const (
	defaultHTTPTimeout = 10 * time.Second
	defaultUserAgent   = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36"
	defaultMaxFeedSize = int64(10 << 20) // 10MiB
)

type Parser interface {
	Parse(reader io.Reader) (*gofeed.Feed, error)
}

type Fetcher interface {
	Fetch(ctx context.Context, feedURL string) (io.ReadCloser, error)
}

type Collector struct {
	parser      Parser
	fetcher     Fetcher
	maxFeedSize int64
}

type Option func(*Collector)

func WithFetcher(fetcher Fetcher) Option {
	return func(c *Collector) {
		if fetcher != nil {
			c.fetcher = fetcher
		}
	}
}

func WithParser(parser Parser) Option {
	return func(c *Collector) {
		if parser != nil {
			c.parser = parser
		}
	}
}

func WithMaxFeedSize(maxFeedSize int64) Option {
	return func(c *Collector) {
		if maxFeedSize > 0 {
			c.maxFeedSize = maxFeedSize
		}
	}
}

func NewCollector(opts ...Option) *Collector {
	c := &Collector{
		fetcher:     NewHTTPFetcher(nil),
		parser:      NewGofeedParser(nil),
		maxFeedSize: defaultMaxFeedSize,
	}

	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *Collector) Type() string {
	return source.TypeRSS
}

func (c *Collector) Collect(ctx context.Context, src *source.Source) ([]collector.CollectedItem, error) {
	if src == nil || src.URL == nil || strings.TrimSpace(*src.URL) == "" {
		return nil, ErrSourceURLRequired
	}

	feedURL := strings.TrimSpace(*src.URL)

	body, err := c.fetcher.Fetch(ctx, feedURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFeedFetchFailed, err)
	}
	defer body.Close()

	reader := io.LimitReader(body, c.maxFeedSize)

	feed, err := c.parser.Parse(reader)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFeedParseFailed, err)
	}

	items := make([]collector.CollectedItem, 0, len(feed.Items))

	for _, item := range feed.Items {
		collected, ok := toCollectedItem(src, item)
		if !ok {
			continue
		}

		items = append(items, collected)
	}
	return items, nil
}

type HTTPFetcher struct {
	client    *http.Client
	userAgent string
}

func NewHTTPFetcher(client *http.Client) *HTTPFetcher {
	if client == nil {
		client = &http.Client{
			Timeout:       defaultHTTPTimeout,
			Transport:     publicHTTPTransport(),
			CheckRedirect: safeFeedRedirect,
		}
	}

	return &HTTPFetcher{
		client:    client,
		userAgent: defaultUserAgent,
	}
}

func (f *HTTPFetcher) Fetch(ctx context.Context, feedURL string) (io.ReadCloser, error) {
	if err := netguard.ValidateHTTPURL(feedURL); err != nil {
		return nil, fmt.Errorf("unsafe feed url: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create feed request: %w", err)
	}

	req.Header.Set("User-Agent", f.userAgent)
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml, text/xml, application/json;q=0.8, */*;q=0.5")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch feed: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return resp.Body, nil
}

func publicHTTPTransport() http.RoundTripper {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return http.DefaultTransport
	}
	clone := transport.Clone()
	clone.DialContext = netguard.DialContext
	return clone
}

func safeFeedRedirect(req *http.Request, via []*http.Request) error {
	if err := netguard.ValidateHTTPURL(req.URL.String()); err != nil {
		return err
	}
	if len(via) >= 10 {
		return http.ErrUseLastResponse
	}
	return nil
}

type GofeedParser struct {
	parser *gofeed.Parser
}

func NewGofeedParser(parser *gofeed.Parser) *GofeedParser {
	if parser == nil {
		parser = gofeed.NewParser()
	}
	return &GofeedParser{
		parser: parser,
	}
}

func (p *GofeedParser) Parse(reader io.Reader) (*gofeed.Feed, error) {
	return p.parser.Parse(reader)
}

func toCollectedItem(src *source.Source, item *gofeed.Item) (collector.CollectedItem, bool) {
	if item == nil {
		return collector.CollectedItem{}, false
	}

	title := strings.TrimSpace(item.Title)
	if title == "" {
		return collector.CollectedItem{}, false
	}

	link := normalizeOptionalString(item.Link)
	content := firstNonEmpty(item.Content, item.Description)
	summary := strings.TrimSpace(item.Description)

	externalID := normalizeExternalID(item)
	contentHash := computeContentHash(src.ID, externalID, title, link, content)

	author := authorName(item)

	return collector.CollectedItem{
		UserID:      src.UserID,
		SourceID:    src.ID,
		SourceType:  src.Type,
		ExternalID:  externalID,
		Title:       title,
		URL:         link,
		OriginalURL: link,
		Author:      author,
		Summary:     summary,
		Content:     strings.TrimSpace(content),
		ContentHash: contentHash,
		PublishedAt: item.PublishedParsed,
	}, true
}
func normalizeExternalID(item *gofeed.Item) *string {
	if item == nil {
		return nil
	}

	candidates := []string{
		item.GUID,
		item.Link,
	}

	for _, candidate := range candidates {
		value := strings.TrimSpace(candidate)
		if value != "" {
			return &value
		}
	}

	return nil
}
func computeContentHash(sourceID int64, externalID *string, title string, url *string, content string) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("source:%d\n", sourceID))

	if externalID != nil {
		b.WriteString("external_id:")
		b.WriteString(strings.TrimSpace(*externalID))
		b.WriteString("\n")
	}

	if url != nil {
		b.WriteString("url:")
		b.WriteString(strings.TrimSpace(*url))
		b.WriteString("\n")
	}

	b.WriteString("title:")
	b.WriteString(strings.TrimSpace(title))
	b.WriteString("\n")

	b.WriteString("content:")
	b.WriteString(normalizeText(content))
	b.WriteString("\n")

	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}
func normalizeOptionalString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}

	return &value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}

	return ""
}

func normalizeText(value string) string {
	fields := strings.Fields(value)
	return strings.Join(fields, " ")
}

func authorName(item *gofeed.Item) string {
	if item == nil || item.Author == nil {
		return ""
	}

	return strings.TrimSpace(item.Author.Name)
}
