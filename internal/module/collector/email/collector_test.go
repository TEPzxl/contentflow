package email

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
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
			name:    "plaintext password is rejected",
			src:     sampleEmailSource(`{"provider":"imap","username":"reader","password":"mail-secret"}`),
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
		{
			name: "skips seen message ids and updates source cursor",
			src: sampleEmailSource(`{
				"provider": "empty",
				"mailbox": "Newsletters",
				"from_filter": "news@example.com",
				"seen_message_ids": ["<old@example.com>"]
			}`),
			reader: &fakeMailboxReader{
				messages: []Message{
					{
						MessageID: "<old@example.com>",
						Subject:   "Old",
						From:      "news@example.com",
						Body:      "old body",
					},
					{
						MessageID: "<new@example.com>",
						Subject:   "New",
						From:      "news@example.com",
						Body:      "new body",
					},
				},
			},
			want: func(t *testing.T, items []collector.CollectedItem) {
				t.Helper()

				if len(items) != 1 {
					t.Fatalf("len(items) = %d, want 1", len(items))
				}
				assertStringPtr(t, "ExternalID", items[0].ExternalID, "<new@example.com>")
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

func TestCollector_Collect_persistsSeenMessageIDs(t *testing.T) {
	src := sampleEmailSource(`{
		"provider": "empty",
		"mailbox": "Newsletters",
		"custom_field": "kept",
		"seen_message_ids": ["<old@example.com>"]
	}`)
	reader := &fakeMailboxReader{
		messages: []Message{
			{
				MessageID: "<old@example.com>",
				Subject:   "Old",
				From:      "news@example.com",
				Body:      "old body",
			},
			{
				MessageID: "<new@example.com>",
				Subject:   "New",
				From:      "news@example.com",
				Body:      "new body",
			},
		},
	}
	c := NewCollector(WithMailboxReader(reader))

	_, err := c.Collect(context.Background(), src)
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	var config map[string]any
	if err := json.Unmarshal(src.ConfigJSON, &config); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if config["custom_field"] != "kept" {
		t.Fatalf("custom_field = %v, want kept", config["custom_field"])
	}

	seen, ok := config["seen_message_ids"].([]any)
	if !ok {
		t.Fatalf("seen_message_ids = %T, want []any", config["seen_message_ids"])
	}
	if len(seen) != 2 {
		t.Fatalf("len(seen_message_ids) = %d, want 2", len(seen))
	}
	if seen[0] != "<old@example.com>" || seen[1] != "<new@example.com>" {
		t.Fatalf("seen_message_ids = %#v", seen)
	}
}

func TestCollector_Collect_persistsLastSeenUID(t *testing.T) {
	src := sampleEmailSource(`{
		"provider": "imap",
		"last_seen_uid": 10
	}`)
	reader := &fakeMailboxReader{
		messages: []Message{
			{
				UID:       11,
				MessageID: "<uid-11@example.com>",
				Subject:   "UID 11",
				From:      "news@example.com",
				Body:      "uid 11 body",
			},
			{
				UID:       13,
				MessageID: "<uid-13@example.com>",
				Subject:   "UID 13",
				From:      "news@example.com",
				Body:      "uid 13 body",
			},
		},
	}
	c := NewCollector(WithMailboxReader(reader))

	_, err := c.Collect(context.Background(), src)
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	var config map[string]any
	if err := json.Unmarshal(src.ConfigJSON, &config); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if config["last_seen_uid"] != float64(13) {
		t.Fatalf("last_seen_uid = %v, want 13", config["last_seen_uid"])
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

func TestDirectoryMailboxReader_Read_decodesTransferEncodingAndEncodedSubject(t *testing.T) {
	dir := t.TempDir()
	writeTestEmail(t, filepath.Join(dir, "encoded.eml"), ""+
		"Message-ID: <encoded@example.com>\r\n"+
		"Subject: =?UTF-8?Q?Weekly_Go_News?=\r\n"+
		"From: news@example.com\r\n"+
		"To: reader@example.com\r\n"+
		"Content-Type: text/plain; charset=utf-8\r\n"+
		"Content-Transfer-Encoding: quoted-printable\r\n"+
		"\r\n"+
		"Hello=20from=20quoted-printable.\r\n")

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
	if messages[0].Subject != "Weekly Go News" {
		t.Fatalf("Subject = %q", messages[0].Subject)
	}
	if messages[0].Body != "Hello from quoted-printable." {
		t.Fatalf("Body = %q", messages[0].Body)
	}
}

func TestDirectoryMailboxReader_Read_convertsHTMLBodyWhenPlainTextMissing(t *testing.T) {
	dir := t.TempDir()
	writeTestEmail(t, filepath.Join(dir, "html.eml"), ""+
		"Message-ID: <html@example.com>\r\n"+
		"Subject: HTML News\r\n"+
		"From: news@example.com\r\n"+
		"To: reader@example.com\r\n"+
		"Content-Type: text/html; charset=utf-8\r\n"+
		"\r\n"+
		"<html><body><h1>Hello</h1><p>from <strong>HTML</strong></p></body></html>\r\n")

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
	if messages[0].Body != "Hello from HTML" {
		t.Fatalf("Body = %q", messages[0].Body)
	}
}

func TestIMAPMailboxReader_Read(t *testing.T) {
	server := newFakeIMAPServer(t, []string{
		"* OK fake imap ready\r\n",
	})
	server.expect(t, "A001 LOGIN \"reader@example.com\" \"secret\"", "A001 OK logged in\r\n")
	server.expect(t, "A002 SELECT \"Newsletters\"", "* 2 EXISTS\r\nA002 OK selected\r\n")
	server.expect(t, "A003 UID SEARCH UID 1:*", "* SEARCH 101\r\nA003 OK search completed\r\n")
	server.expect(t, "A004 UID FETCH 101 RFC822", "* 1 FETCH (UID 101 RFC822 {159}\r\n"+
		"Message-ID: <imap@example.com>\r\n"+
		"Subject: IMAP News\r\n"+
		"From: news@example.com\r\n"+
		"To: reader@example.com\r\n"+
		"Date: Wed, 13 May 2026 12:00:00 +0000\r\n"+
		"\r\n"+
		"Hello from IMAP.\r\n"+
		")\r\n"+
		"A004 OK fetch completed\r\n")
	server.expect(t, "A005 LOGOUT", "* BYE logging out\r\nA005 OK logout completed\r\n")

	reader := NewIMAPMailboxReader(WithIMAPDialer(func(ctx context.Context, network string, address string) (net.Conn, error) {
		return net.Dial(network, server.address)
	}))

	messages, err := reader.Read(context.Background(), Config{
		Provider: "imap",
		Host:     "imap.example.com",
		Port:     143,
		Username: "reader@example.com",
		Password: "secret",
		Mailbox:  "Newsletters",
		UseTLS:   boolPtr(false),
	})
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(messages))
	}
	if messages[0].MessageID != "<imap@example.com>" {
		t.Fatalf("MessageID = %q", messages[0].MessageID)
	}
	if messages[0].UID != 101 {
		t.Fatalf("UID = %d, want 101", messages[0].UID)
	}
	if messages[0].Subject != "IMAP News" {
		t.Fatalf("Subject = %q", messages[0].Subject)
	}
	if messages[0].Body != "Hello from IMAP." {
		t.Fatalf("Body = %q", messages[0].Body)
	}
}

func TestIMAPMailboxReader_ReadRejectsUnsafeHostBeforeDial(t *testing.T) {
	reader := NewIMAPMailboxReader(WithIMAPDialer(func(ctx context.Context, network string, address string) (net.Conn, error) {
		t.Fatalf("dialer called for unsafe address %s", address)
		return nil, nil
	}))

	_, err := reader.Read(context.Background(), Config{
		Provider: "imap",
		Host:     "127.0.0.1",
		Port:     143,
		Username: "reader@example.com",
		Password: "secret",
		Mailbox:  "INBOX",
		UseTLS:   boolPtr(false),
	})
	if err == nil {
		t.Fatal("Read() expected unsafe host error, got nil")
	}
}

func TestIMAPMailboxReader_Read_searchesAfterLastSeenUID(t *testing.T) {
	server := newFakeIMAPServer(t, []string{
		"* OK fake imap ready\r\n",
	})
	server.expect(t, "A001 LOGIN \"reader@example.com\" \"secret\"", "A001 OK logged in\r\n")
	server.expect(t, "A002 SELECT \"INBOX\"", "* 0 EXISTS\r\nA002 OK selected\r\n")
	server.expect(t, "A003 UID SEARCH UID 43:*", "* SEARCH\r\nA003 OK search completed\r\n")
	server.expect(t, "A004 LOGOUT", "* BYE logging out\r\nA004 OK logout completed\r\n")

	reader := NewIMAPMailboxReader(WithIMAPDialer(func(ctx context.Context, network string, address string) (net.Conn, error) {
		return net.Dial(network, server.address)
	}))

	messages, err := reader.Read(context.Background(), Config{
		Provider:    "imap",
		Host:        "imap.example.com",
		Port:        143,
		Username:    "reader@example.com",
		Password:    "secret",
		Mailbox:     "INBOX",
		UseTLS:      boolPtr(false),
		LastSeenUID: 42,
	})
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if len(messages) != 0 {
		t.Fatalf("len(messages) = %d, want 0", len(messages))
	}
}

func TestIMAPMailboxReader_Read_usesPasswordEnv(t *testing.T) {
	t.Setenv("CONTENTFLOW_TEST_IMAP_PASSWORD", "secret-from-env")

	server := newFakeIMAPServer(t, []string{
		"* OK fake imap ready\r\n",
	})
	server.expect(t, "A001 LOGIN \"reader@example.com\" \"secret-from-env\"", "A001 OK logged in\r\n")
	server.expect(t, "A002 SELECT \"INBOX\"", "* 0 EXISTS\r\nA002 OK selected\r\n")
	server.expect(t, "A003 UID SEARCH UID 1:*", "* SEARCH\r\nA003 OK search completed\r\n")
	server.expect(t, "A004 LOGOUT", "* BYE logging out\r\nA004 OK logout completed\r\n")

	reader := NewIMAPMailboxReader(WithIMAPDialer(func(ctx context.Context, network string, address string) (net.Conn, error) {
		return net.Dial(network, server.address)
	}))

	_, err := reader.Read(context.Background(), Config{
		Provider:    "imap",
		Host:        "imap.example.com",
		Port:        143,
		Username:    "reader@example.com",
		PasswordEnv: "CONTENTFLOW_TEST_IMAP_PASSWORD",
		Mailbox:     "INBOX",
		UseTLS:      boolPtr(false),
	})
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
}

func TestConfiguredMailboxReader_Read_supportsIMAPProvider(t *testing.T) {
	server := newFakeIMAPServer(t, []string{
		"* OK fake imap ready\r\n",
	})
	server.expect(t, "A001 LOGIN \"reader@example.com\" \"secret\"", "A001 OK logged in\r\n")
	server.expect(t, "A002 SELECT \"INBOX\"", "* 1 EXISTS\r\nA002 OK selected\r\n")
	server.expect(t, "A003 UID SEARCH UID 1:*", "* SEARCH 201\r\nA003 OK search completed\r\n")
	server.expect(t, "A004 UID FETCH 201 RFC822", "* 1 FETCH (UID 201 RFC822 {121}\r\n"+
		"Message-ID: <configured@example.com>\r\n"+
		"Subject: Configured IMAP\r\n"+
		"From: news@example.com\r\n"+
		"To: reader@example.com\r\n"+
		"\r\n"+
		"Body.\r\n"+
		")\r\n"+
		"A004 OK fetch completed\r\n")
	server.expect(t, "A005 LOGOUT", "* BYE logging out\r\nA005 OK logout completed\r\n")

	reader := &ConfiguredMailboxReader{
		directoryReader: NewDirectoryMailboxReader(),
		imapReader: NewIMAPMailboxReader(WithIMAPDialer(func(ctx context.Context, network string, address string) (net.Conn, error) {
			return net.Dial(network, server.address)
		})),
		emptyReader: emptyMailboxReader{},
	}

	messages, err := reader.Read(context.Background(), Config{
		Provider: "imap",
		Host:     "imap.example.com",
		Port:     143,
		Username: "reader@example.com",
		Password: "secret",
		Mailbox:  "INBOX",
		UseTLS:   boolPtr(false),
	})
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(messages))
	}
	if messages[0].Subject != "Configured IMAP" {
		t.Fatalf("Subject = %q", messages[0].Subject)
	}
}

func writeTestEmail(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write test email: %v", err)
	}
}

func boolPtr(value bool) *bool {
	return &value
}

type fakeIMAPServer struct {
	address string
	steps   chan fakeIMAPStep
	done    chan struct{}
}

type fakeIMAPStep struct {
	command  string
	response string
}

func newFakeIMAPServer(t *testing.T, greetings []string) *fakeIMAPServer {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen fake imap: %v", err)
	}

	server := &fakeIMAPServer{
		address: listener.Addr().String(),
		steps:   make(chan fakeIMAPStep, 16),
		done:    make(chan struct{}),
	}

	go func() {
		defer close(server.done)
		defer listener.Close()

		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		for _, greeting := range greetings {
			_, _ = conn.Write([]byte(greeting))
		}

		reader := bufio.NewReader(conn)
		for step := range server.steps {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimRight(line, "\r\n")
			if line != step.command {
				t.Errorf("imap command = %q, want %q", line, step.command)
				return
			}
			_, _ = conn.Write([]byte(step.response))
		}
	}()

	t.Cleanup(func() {
		close(server.steps)
		<-server.done
	})

	return server
}

func (s *fakeIMAPServer) expect(t *testing.T, command string, response string) {
	t.Helper()
	s.steps <- fakeIMAPStep{command: command, response: response}
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
