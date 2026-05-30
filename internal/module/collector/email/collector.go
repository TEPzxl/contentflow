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
	Provider       string   `json:"provider"`
	Mailbox        string   `json:"mailbox"`
	Host           string   `json:"host"`
	Port           int      `json:"port"`
	Username       string   `json:"username"`
	Password       string   `json:"password"`
	PasswordEnv    string   `json:"password_env"`
	UseTLS         *bool    `json:"use_tls"`
	FromFilter     string   `json:"from_filter"`
	RecipientAlias string   `json:"recipient_alias"`
	SeenMessageIDs []string `json:"seen_message_ids"`
	LastSeenUID    int      `json:"last_seen_uid"`
}

type Message struct {
	UID       int
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
		reader: NewConfiguredMailboxReader(),
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
	seen := seenMessageIDSet(cfg.SeenMessageIDs)
	newMessageIDs := make([]string, 0, len(messages))
	lastSeenUID := cfg.LastSeenUID
	for _, msg := range messages {
		messageID := strings.TrimSpace(msg.MessageID)
		if messageID != "" && seen[messageID] {
			continue
		}

		if !matchesFilters(msg, cfg) {
			continue
		}

		item, ok := toCollectedItem(src, msg)
		if !ok {
			continue
		}

		items = append(items, item)
		if messageID != "" {
			newMessageIDs = append(newMessageIDs, messageID)
		}
		if msg.UID > lastSeenUID {
			lastSeenUID = msg.UID
		}
	}

	if len(newMessageIDs) > 0 {
		if err := updateSeenMessageIDs(src, cfg.SeenMessageIDs, newMessageIDs); err != nil {
			return nil, err
		}
	}
	if lastSeenUID > cfg.LastSeenUID {
		if err := updateLastSeenUID(src, lastSeenUID); err != nil {
			return nil, err
		}
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

	cfg.Provider = strings.TrimSpace(strings.ToLower(cfg.Provider))
	cfg.Mailbox = strings.TrimSpace(cfg.Mailbox)
	cfg.Host = strings.TrimSpace(cfg.Host)
	cfg.Username = strings.TrimSpace(cfg.Username)
	cfg.PasswordEnv = strings.TrimSpace(cfg.PasswordEnv)
	cfg.FromFilter = strings.TrimSpace(cfg.FromFilter)
	cfg.RecipientAlias = strings.TrimSpace(cfg.RecipientAlias)
	cfg.SeenMessageIDs = normalizeSeenMessageIDs(cfg.SeenMessageIDs)
	if cfg.LastSeenUID < 0 {
		cfg.LastSeenUID = 0
	}

	return cfg, nil
}

func seenMessageIDSet(ids []string) map[string]bool {
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			seen[id] = true
		}
	}
	return seen
}

func normalizeSeenMessageIDs(ids []string) []string {
	normalized := make([]string, 0, len(ids))
	seen := map[string]bool{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		normalized = append(normalized, id)
	}
	return normalized
}

func updateSeenMessageIDs(src *source.Source, existing []string, newIDs []string) error {
	if src == nil {
		return nil
	}

	config := map[string]any{}
	if len(src.ConfigJSON) > 0 {
		if err := json.Unmarshal(src.ConfigJSON, &config); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidEmailConfig, err)
		}
	}

	merged := normalizeSeenMessageIDs(append(existing, newIDs...))
	config["seen_message_ids"] = merged

	data, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidEmailConfig, err)
	}
	src.ConfigJSON = data
	return nil
}

func updateLastSeenUID(src *source.Source, uid int) error {
	if src == nil || uid <= 0 {
		return nil
	}

	config := map[string]any{}
	if len(src.ConfigJSON) > 0 {
		if err := json.Unmarshal(src.ConfigJSON, &config); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidEmailConfig, err)
		}
	}

	config["last_seen_uid"] = uid
	data, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidEmailConfig, err)
	}
	src.ConfigJSON = data
	return nil
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
		UserID:      src.UserID,
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

type ConfiguredMailboxReader struct {
	directoryReader MailboxReader
	imapReader      MailboxReader
	emptyReader     MailboxReader
}

func NewConfiguredMailboxReader() *ConfiguredMailboxReader {
	return &ConfiguredMailboxReader{
		directoryReader: NewDirectoryMailboxReader(),
		imapReader:      NewIMAPMailboxReader(),
		emptyReader:     emptyMailboxReader{},
	}
}

func (r *ConfiguredMailboxReader) Read(ctx context.Context, cfg Config) ([]Message, error) {
	switch cfg.Provider {
	case "", "empty":
		return r.emptyReader.Read(ctx, cfg)
	case "directory":
		return r.directoryReader.Read(ctx, cfg)
	case "imap":
		return r.imapReader.Read(ctx, cfg)
	default:
		return nil, ErrInvalidEmailConfig
	}
}
