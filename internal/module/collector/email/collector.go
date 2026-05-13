package email

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tepzxl/contentflow/internal/module/collector"
	"github.com/tepzxl/contentflow/internal/module/source"
)

var (
	ErrInvalidEmailConfig = errors.New("invalid email config")
	ErrMailboxReadFailed  = errors.New("mailbox read failed")
)

type Config struct {
	Mailbox        string `json:"mailbox"`
	FromFilter     string `json:"from_filter"`
	RecipientAlias string `json:"recipient_alias"`
}

type Message struct {
	MessageID string
	Subject   string
	From      string
	To        []string
	Body      string
	Date      *time.Time
}

type MailboxReader interface {
	Read(ctx context.Context, cfg Config) ([]Message, error)
}

type Collector struct {
	reader MailboxReader
}

type Option func(*Collector)

func WithMailboxReader(reader MailboxReader) Option {
	return func(c *Collector) {
		if reader != nil {
			c.reader = reader
		}
	}
}

func NewCollector(opts ...Option) *Collector {
	c := &Collector{
		reader: emptyMailboxReader{},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (c *Collector) Type() string {
	return source.TypeEmail
}

func (c *Collector) Collect(ctx context.Context, src *source.Source) ([]collector.CollectedItem, error) {
	cfg, err := parseConfig(src)
	if err != nil {
		return nil, err
	}

	messages, err := c.reader.Read(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMailboxReadFailed, err)
	}

	items := make([]collector.CollectedItem, 0, len(messages))
	for _, msg := range messages {
		if !matchesFilters(msg, cfg) {
			continue
		}

		item, ok := toCollectedItem(src, msg)
		if !ok {
			continue
		}

		items = append(items, item)
	}

	return items, nil
}

func parseConfig(src *source.Source) (Config, error) {
	var cfg Config
	if src == nil || len(src.ConfigJSON) == 0 {
		return cfg, nil
	}

	if err := json.Unmarshal(src.ConfigJSON, &cfg); err != nil {
		return Config{}, fmt.Errorf("%w: %v", ErrInvalidEmailConfig, err)
	}

	cfg.Mailbox = strings.TrimSpace(cfg.Mailbox)
	cfg.FromFilter = strings.TrimSpace(cfg.FromFilter)
	cfg.RecipientAlias = strings.TrimSpace(cfg.RecipientAlias)

	return cfg, nil
}

func matchesFilters(msg Message, cfg Config) bool {
	if cfg.FromFilter != "" && !containsFold(msg.From, cfg.FromFilter) {
		return false
	}

	if cfg.RecipientAlias != "" {
		for _, recipient := range msg.To {
			if containsFold(recipient, cfg.RecipientAlias) {
				return true
			}
		}
		return false
	}

	return true
}

func toCollectedItem(src *source.Source, msg Message) (collector.CollectedItem, bool) {
	title := strings.TrimSpace(msg.Subject)
	if title == "" {
		return collector.CollectedItem{}, false
	}

	externalID := normalizeOptionalString(msg.MessageID)
	content := strings.TrimSpace(msg.Body)
	author := strings.TrimSpace(msg.From)

	return collector.CollectedItem{
		SourceID:    src.ID,
		SourceType:  src.Type,
		ExternalID:  externalID,
		Title:       title,
		URL:         nil,
		OriginalURL: nil,
		Author:      author,
		Summary:     "",
		Content:     content,
		ContentHash: computeContentHash(src.ID, externalID, title, author, content),
		PublishedAt: msg.Date,
	}, true
}

func computeContentHash(sourceID int64, externalID *string, title string, author string, content string) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("source:%d\n", sourceID))

	if externalID != nil {
		b.WriteString("external_id:")
		b.WriteString(strings.TrimSpace(*externalID))
		b.WriteString("\n")
	}

	b.WriteString("title:")
	b.WriteString(strings.TrimSpace(title))
	b.WriteString("\n")

	b.WriteString("author:")
	b.WriteString(strings.TrimSpace(author))
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

func containsFold(value string, substr string) bool {
	return strings.Contains(strings.ToLower(value), strings.ToLower(substr))
}

func normalizeText(value string) string {
	fields := strings.Fields(value)
	return strings.Join(fields, " ")
}

type emptyMailboxReader struct{}

func (emptyMailboxReader) Read(ctx context.Context, cfg Config) ([]Message, error) {
	return []Message{}, nil
}
