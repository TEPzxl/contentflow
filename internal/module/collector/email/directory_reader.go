package email

import (
	"context"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type DirectoryMailboxReader struct{}

func NewDirectoryMailboxReader() *DirectoryMailboxReader {
	return &DirectoryMailboxReader{}
}

func (r *DirectoryMailboxReader) Read(ctx context.Context, cfg Config) ([]Message, error) {
	mailbox := strings.TrimSpace(cfg.Mailbox)
	if mailbox == "" {
		return nil, ErrInvalidEmailConfig
	}

	entries, err := os.ReadDir(mailbox)
	if err != nil {
		return nil, fmt.Errorf("read mailbox directory: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	messages := make([]Message, 0, len(names))
	for _, name := range names {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		msg, err := readEmailFile(filepath.Join(mailbox, name))
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

func readEmailFile(path string) (Message, error) {
	file, err := os.Open(path)
	if err != nil {
		return Message{}, fmt.Errorf("open email file: %w", err)
	}
	defer file.Close()

	msg, err := mail.ReadMessage(file)
	if err != nil {
		return Message{}, fmt.Errorf("parse email message: %w", err)
	}

	body, err := readMessageBody(msg.Header, msg.Body)
	if err != nil {
		return Message{}, err
	}

	parsedDate, err := msg.Header.Date()
	var date *time.Time
	if err == nil {
		parsedDate = parsedDate.UTC()
		date = &parsedDate
	}

	return Message{
		MessageID: strings.TrimSpace(msg.Header.Get("Message-ID")),
		Subject:   strings.TrimSpace(msg.Header.Get("Subject")),
		From:      strings.TrimSpace(msg.Header.Get("From")),
		To:        parseAddressList(msg.Header.Get("To")),
		Body:      strings.TrimSpace(body),
		Date:      date,
	}, nil
}

func readMessageBody(header mail.Header, body io.Reader) (string, error) {
	contentType := header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || !strings.HasPrefix(strings.ToLower(mediaType), "multipart/") {
		data, readErr := io.ReadAll(body)
		if readErr != nil {
			return "", fmt.Errorf("read email body: %w", readErr)
		}
		return string(data), nil
	}

	boundary := params["boundary"]
	if boundary == "" {
		return "", ErrInvalidEmailConfig
	}

	mr := multipart.NewReader(body, boundary)
	var firstText string
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("read multipart email: %w", err)
		}

		data, err := io.ReadAll(part)
		if err != nil {
			return "", fmt.Errorf("read email part: %w", err)
		}

		partMediaType, _, _ := mime.ParseMediaType(part.Header.Get("Content-Type"))
		partMediaType = strings.ToLower(strings.TrimSpace(partMediaType))
		if partMediaType == "text/plain" {
			return string(data), nil
		}
		if firstText == "" && strings.HasPrefix(partMediaType, "text/") {
			firstText = string(data)
		}
	}

	return firstText, nil
}

func parseAddressList(value string) []string {
	addresses, err := mail.ParseAddressList(value)
	if err != nil {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil
		}
		return []string{value}
	}

	result := make([]string, 0, len(addresses))
	for _, address := range addresses {
		result = append(result, address.String())
	}
	return result
}
