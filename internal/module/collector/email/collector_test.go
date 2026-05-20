package email

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tepzxl/contentflow/internal/module/collector"
	"github.com/tepzxl/contentflow/internal/module/source"
	"gorm.io/datatypes"
)

var errMailbox = errors.New("mailbox error")

func TestCollector_Collect(t *testing.T) {
	receivedAt := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		src     *source.Source
		reader  *fakeMailboxReader
		want    func(t *testing.T, items []collector.CollectedItem)
		wantErr error
	}{
		{
			name: "invalid config",
			src: &source.Source{
				ID:         1,
				Type:       source.TypeEmail,
				ConfigJSON: datatypes.JSON([]byte(`{`)),
			},
			reader:  &fakeMailboxReader{},
			wantErr: ErrInvalidEmailConfig,
		},
		{
			name: "mailbox reader failure is wrapped",
			src:  sampleEmailSource(`{"mailbox":"INBOX"}`),
			reader: &fakeMailboxReader{
				err: errMailbox,
			},
			wantErr: ErrMailboxReadFailed,
		},
		{
			name: "message fields are converted",
			src:  sampleEmailSource(`{"mailbox":"Newsletters"}`),
			reader: &fakeMailboxReader{
				messages: []Message{
					{
						MessageID: "<msg-1@example.com>",
						Subject:   " Weekly Go News ",
						From:      "Go Newsletter <news@example.com>",
						To:        []string{"reader@example.com"},
						Body:      " Hello from Go newsletter ",
						Date:      &receivedAt,
					},
					{
						MessageID: "<empty-subject@example.com>",
						Subject:   "  ",
						From:      "news@example.com",
						Body:      "skipped",
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
				if item.SourceType != source.TypeEmail {
					t.Fatalf("SourceType = %q, want %q", item.SourceType, source.TypeEmail)
				}
				assertStringPtr(t, "ExternalID", item.ExternalID, "<msg-1@example.com>")
				if item.Title != "Weekly Go News" {
					t.Fatalf("Title = %q, want Weekly Go News", item.Title)
				}
				if item.Author != "Go Newsletter <news@example.com>" {
					t.Fatalf("Author = %q", item.Author)
				}
				if item.Summary != "" {
					t.Fatalf("Summary = %q, want empty", item.Summary)
				}
				if item.Content != "Hello from Go newsletter" {
					t.Fatalf("Content = %q", item.Content)
				}
				if item.ContentHash == "" || len(item.ContentHash) != 64 {
					t.Fatalf("ContentHash = %q, want 64-char hash", item.ContentHash)
				}
				if item.PublishedAt == nil || !item.PublishedAt.Equal(receivedAt) {
					t.Fatalf("PublishedAt = %v, want %v", item.PublishedAt, receivedAt)
				}
			},
		},
		{
			name: "filters by from and recipient alias",
			src: sampleEmailSource(`{
				"mailbox": "Newsletters",
				"from_filter": "news@example.com",
				"recipient_alias": "reader+go@example.com"
			}`),
			reader: &fakeMailboxReader{
				messages: []Message{
					{
						MessageID: "<match@example.com>",
						Subject:   "Matched",
						From:      "News <news@example.com>",
						To:        []string{"reader+go@example.com"},
						Body:      "matched body",
					},
					{
						MessageID: "<wrong-from@example.com>",
						Subject:   "Wrong From",
						From:      "Other <other@example.com>",
						To:        []string{"reader+go@example.com"},
						Body:      "wrong from",
					},
					{
						MessageID: "<wrong-to@example.com>",
						Subject:   "Wrong To",
						From:      "News <news@example.com>",
						To:        []string{"reader+other@example.com"},
						Body:      "wrong to",
					},
				},
			},
			want: func(t *testing.T, items []collector.CollectedItem) {
				t.Helper()

				if len(items) != 1 {
					t.Fatalf("len(items) = %d, want 1", len(items))
				}
				assertStringPtr(t, "ExternalID", items[0].ExternalID, "<match@example.com>")
			},
		},
		{
			name: "content hash is stable for same normalized content",
			src:  sampleEmailSource(`{}`),
			reader: &fakeMailboxReader{
				messages: []Message{
					{
						MessageID: "<msg-1@example.com>",
						Subject:   "Weekly Go News",
						From:      "news@example.com",
						Body:      "Hello   from\nGo",
					},
					{
						MessageID: "<msg-1@example.com>",
						Subject:   " Weekly Go News ",
						From:      " news@example.com ",
						Body:      "Hello from Go",
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
			c := NewCollector(WithMailboxReader(tt.reader))

			items, err := c.Collect(context.Background(), tt.src)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Collect() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("Collect() unexpected error = %v", err)
			}
			if tt.want != nil {
				tt.want(t, items)
			}
		})
	}
}

func TestCollector_Type(t *testing.T) {
	c := NewCollector()

	if got := c.Type(); got != source.TypeEmail {
		t.Fatalf("Type() = %q, want %q", got, source.TypeEmail)
	}
}

func TestCollector_Collect_defaultReaderSupportsDirectoryProvider(t *testing.T) {
	dir := t.TempDir()
	writeTestEmail(t, filepath.Join(dir, "weekly.eml"), ""+
		"Message-ID: <default-reader@example.com>\r\n"+
		"Subject: Default Reader News\r\n"+
		"From: news@example.com\r\n"+
		"To: reader@example.com\r\n"+
		"Content-Type: text/plain; charset=utf-8\r\n"+
		"\r\n"+
		"Default reader body.\r\n")

	c := NewCollector()

	items, err := c.Collect(context.Background(), sampleEmailSource(`{
		"provider": "directory",
		"mailbox": "`+dir+`"
	}`))
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].Title != "Default Reader News" {
		t.Fatalf("Title = %q", items[0].Title)
	}
	if items[0].Content != "Default reader body." {
		t.Fatalf("Content = %q", items[0].Content)
	}
}

func TestDirectoryMailboxReader_Read(t *testing.T) {
	dir := t.TempDir()
	writeTestEmail(t, filepath.Join(dir, "weekly.eml"), ""+
		"Message-ID: <weekly@example.com>\r\n"+
		"Subject: Weekly Go News\r\n"+
		"From: Go Newsletter <news@example.com>\r\n"+
		"To: Reader <reader@example.com>, reader+go@example.com\r\n"+
		"Date: Wed, 13 May 2026 12:00:00 +0000\r\n"+
		"Content-Type: text/plain; charset=utf-8\r\n"+
		"\r\n"+
		"Hello from Go newsletter.\r\n")

	reader := NewDirectoryMailboxReader()

	messages, err := reader.Read(context.Background(), Config{
		Provider: "directory",
		Mailbox:  dir,
	})
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(messages))
	}

	msg := messages[0]
	if msg.MessageID != "<weekly@example.com>" {
		t.Fatalf("MessageID = %q", msg.MessageID)
	}
	if msg.Subject != "Weekly Go News" {
		t.Fatalf("Subject = %q", msg.Subject)
	}
	if msg.From != "Go Newsletter <news@example.com>" {
		t.Fatalf("From = %q", msg.From)
	}
	if len(msg.To) != 2 {
		t.Fatalf("len(To) = %d, want 2", len(msg.To))
	}
	if msg.Body != "Hello from Go newsletter." {
		t.Fatalf("Body = %q", msg.Body)
	}
	if msg.Date == nil || !msg.Date.Equal(time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("Date = %v", msg.Date)
	}
}

func TestDirectoryMailboxReader_Read_prefersPlainTextPart(t *testing.T) {
	dir := t.TempDir()
	writeTestEmail(t, filepath.Join(dir, "multipart.eml"), ""+
		"Message-ID: <multipart@example.com>\r\n"+
		"Subject: Multipart News\r\n"+
		"From: news@example.com\r\n"+
		"To: reader@example.com\r\n"+
		"Content-Type: multipart/alternative; boundary=frontier\r\n"+
		"\r\n"+
		"--frontier\r\n"+
		"Content-Type: text/html; charset=utf-8\r\n"+
		"\r\n"+
		"<p>HTML body</p>\r\n"+
		"--frontier\r\n"+
		"Content-Type: text/plain; charset=utf-8\r\n"+
		"\r\n"+
		"Plain body\r\n"+
		"--frontier--\r\n")

	reader := NewDirectoryMailboxReader()

	messages, err := reader.Read(context.Background(), Config{
		Provider: "directory",
		Mailbox:  dir,
	})
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(messages))
	}
	if messages[0].Body != "Plain body" {
		t.Fatalf("Body = %q", messages[0].Body)
	}
}

func writeTestEmail(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write test email: %v", err)
	}
}

type fakeMailboxReader struct {
	config   Config
	messages []Message
	err      error
}

func (r *fakeMailboxReader) Read(ctx context.Context, cfg Config) ([]Message, error) {
	r.config = cfg
	if r.err != nil {
		return nil, r.err
	}
	return r.messages, nil
}

func sampleEmailSource(config string) *source.Source {
	return &source.Source{
		ID:         10,
		UserID:     100,
		Name:       "Newsletter",
		Type:       source.TypeEmail,
		ConfigJSON: datatypes.JSON([]byte(config)),
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
