package email

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type IMAPDialer func(ctx context.Context, network string, address string) (net.Conn, error)

type IMAPMailboxReader struct {
	dialer IMAPDialer
}

type IMAPOption func(*IMAPMailboxReader)

func WithIMAPDialer(dialer IMAPDialer) IMAPOption {
	return func(r *IMAPMailboxReader) {
		if dialer != nil {
			r.dialer = dialer
		}
	}
}

func NewIMAPMailboxReader(opts ...IMAPOption) *IMAPMailboxReader {
	r := &IMAPMailboxReader{
		dialer: defaultIMAPDialer,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func defaultIMAPDialer(ctx context.Context, network string, address string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	return dialer.DialContext(ctx, network, address)
}

func (r *IMAPMailboxReader) Read(ctx context.Context, cfg Config) ([]Message, error) {
	if err := validateIMAPConfig(cfg); err != nil {
		return nil, err
	}

	conn, err := r.dial(ctx, cfg)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := newIMAPClient(conn)
	if err := client.readGreeting(); err != nil {
		return nil, err
	}
	password, err := imapPassword(cfg)
	if err != nil {
		return nil, err
	}
	if err := client.commandOK("LOGIN %s %s", quoteIMAPString(cfg.Username), quoteIMAPString(password)); err != nil {
		return nil, err
	}

	mailbox := cfg.Mailbox
	if strings.TrimSpace(mailbox) == "" {
		mailbox = "INBOX"
	}
	if err := client.commandOK("SELECT %s", quoteIMAPString(mailbox)); err != nil {
		return nil, err
	}

	ids, err := client.searchUIDs(cfg.LastSeenUID)
	if err != nil {
		return nil, err
	}

	messages := make([]Message, 0, len(ids))
	for _, id := range ids {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		msg, err := client.fetchUIDRFC822(id)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	_ = client.commandOK("LOGOUT")
	return messages, nil
}

func validateIMAPConfig(cfg Config) error {
	if strings.TrimSpace(cfg.Host) == "" || strings.TrimSpace(cfg.Username) == "" {
		return ErrInvalidEmailConfig
	}
	if strings.TrimSpace(cfg.Password) == "" && strings.TrimSpace(cfg.PasswordEnv) == "" {
		return ErrInvalidEmailConfig
	}
	return nil
}

func imapPassword(cfg Config) (string, error) {
	if strings.TrimSpace(cfg.Password) != "" {
		return cfg.Password, nil
	}

	envName := strings.TrimSpace(cfg.PasswordEnv)
	if envName == "" {
		return "", ErrInvalidEmailConfig
	}

	password := strings.TrimSpace(os.Getenv(envName))
	if password == "" {
		return "", ErrInvalidEmailConfig
	}
	return password, nil
}

func (r *IMAPMailboxReader) dial(ctx context.Context, cfg Config) (net.Conn, error) {
	port := cfg.Port
	if port <= 0 {
		if useTLS(cfg) {
			port = 993
		} else {
			port = 143
		}
	}

	address := net.JoinHostPort(cfg.Host, strconv.Itoa(port))
	if !useTLS(cfg) {
		conn, err := r.dialer(ctx, "tcp", address)
		if err != nil {
			return nil, fmt.Errorf("dial imap: %w", err)
		}
		return conn, nil
	}

	dialer := tls.Dialer{
		NetDialer: &net.Dialer{Timeout: 10 * time.Second},
		Config:    &tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12},
	}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("dial imaps: %w", err)
	}
	return conn, nil
}

func useTLS(cfg Config) bool {
	if cfg.UseTLS != nil {
		return *cfg.UseTLS
	}
	return true
}

type imapClient struct {
	conn   net.Conn
	reader *bufio.Reader
	nextID int
}

func newIMAPClient(conn net.Conn) *imapClient {
	return &imapClient{
		conn:   conn,
		reader: bufio.NewReader(conn),
		nextID: 1,
	}
}

func (c *imapClient) readGreeting() error {
	line, err := c.readLine()
	if err != nil {
		return err
	}
	if !strings.HasPrefix(line, "* OK") {
		return fmt.Errorf("imap greeting failed: %s", line)
	}
	return nil
}

func (c *imapClient) commandOK(format string, args ...any) error {
	tag := c.nextTag()
	if err := c.writeCommand(tag, format, args...); err != nil {
		return err
	}
	for {
		line, err := c.readLine()
		if err != nil {
			return err
		}
		if strings.HasPrefix(line, tag+" OK") {
			return nil
		}
		if strings.HasPrefix(line, tag+" NO") || strings.HasPrefix(line, tag+" BAD") {
			return fmt.Errorf("imap command failed: %s", line)
		}
	}
}

func (c *imapClient) searchUIDs(lastSeenUID int) ([]string, error) {
	tag := c.nextTag()
	startUID := lastSeenUID + 1
	if startUID <= 0 {
		startUID = 1
	}
	if err := c.writeCommand(tag, "UID SEARCH UID %d:*", startUID); err != nil {
		return nil, err
	}

	var ids []string
	for {
		line, err := c.readLine()
		if err != nil {
			return nil, err
		}
		if strings.HasPrefix(line, "* SEARCH") {
			fields := strings.Fields(line)
			if len(fields) > 2 {
				ids = append(ids, fields[2:]...)
			}
			continue
		}
		if strings.HasPrefix(line, tag+" OK") {
			return ids, nil
		}
		if strings.HasPrefix(line, tag+" NO") || strings.HasPrefix(line, tag+" BAD") {
			return nil, fmt.Errorf("imap search failed: %s", line)
		}
	}
}

func (c *imapClient) fetchUIDRFC822(id string) (Message, error) {
	tag := c.nextTag()
	if err := c.writeCommand(tag, "UID FETCH %s RFC822", id); err != nil {
		return Message{}, err
	}

	var raw []byte
	uid, err := strconv.Atoi(id)
	if err != nil {
		return Message{}, fmt.Errorf("parse imap uid: %w", err)
	}
	for {
		line, err := c.readLine()
		if err != nil {
			return Message{}, err
		}

		if strings.HasPrefix(line, "* ") && strings.Contains(line, "RFC822 {") {
			size, err := literalSize(line)
			if err != nil {
				return Message{}, err
			}
			raw = make([]byte, size)
			if _, err := c.reader.Read(raw); err != nil {
				return Message{}, fmt.Errorf("read imap literal: %w", err)
			}
			continue
		}

		if strings.HasPrefix(line, tag+" OK") {
			if len(raw) == 0 {
				return Message{}, fmt.Errorf("imap fetch returned empty message")
			}
			msg, err := readEmail(bytes.NewReader(raw))
			if err != nil {
				return Message{}, err
			}
			msg.UID = uid
			return msg, nil
		}
		if strings.HasPrefix(line, tag+" NO") || strings.HasPrefix(line, tag+" BAD") {
			return Message{}, fmt.Errorf("imap fetch failed: %s", line)
		}
	}
}

func (c *imapClient) nextTag() string {
	tag := fmt.Sprintf("A%03d", c.nextID)
	c.nextID++
	return tag
}

func (c *imapClient) writeCommand(tag string, format string, args ...any) error {
	line := tag + " " + fmt.Sprintf(format, args...) + "\r\n"
	if _, err := c.conn.Write([]byte(line)); err != nil {
		return fmt.Errorf("write imap command: %w", err)
	}
	return nil
}

func (c *imapClient) readLine() (string, error) {
	line, err := c.reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("read imap response: %w", err)
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func quoteIMAPString(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`
}

func literalSize(line string) (int, error) {
	start := strings.LastIndex(line, "{")
	end := strings.LastIndex(line, "}")
	if start < 0 || end <= start {
		return 0, fmt.Errorf("imap literal size missing: %s", line)
	}

	size, err := strconv.Atoi(line[start+1 : end])
	if err != nil {
		return 0, fmt.Errorf("parse imap literal size: %w", err)
	}
	return size, nil
}
