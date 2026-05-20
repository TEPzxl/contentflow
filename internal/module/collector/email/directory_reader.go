package email

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/html"
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

	return readEmail(file)
}

func readEmail(reader io.Reader) (Message, error) {
	msg, err := mail.ReadMessage(reader)
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
		Subject:   strings.TrimSpace(decodeHeader(msg.Header.Get("Subject"))),
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
		content, readErr := readDecodedBody(body, header.Get("Content-Transfer-Encoding"))
		if readErr != nil {
			return "", readErr
		}
		if strings.EqualFold(mediaType, "text/html") {
			return htmlToText(content), nil
		}
		return content, nil
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

		content, err := readDecodedBody(part, part.Header.Get("Content-Transfer-Encoding"))
		if err != nil {
			return "", err
		}

		partMediaType, _, _ := mime.ParseMediaType(part.Header.Get("Content-Type"))
		partMediaType = strings.ToLower(strings.TrimSpace(partMediaType))
		if partMediaType == "text/plain" {
			return content, nil
		}
		if firstText == "" && partMediaType == "text/html" {
			firstText = htmlToText(content)
		} else if firstText == "" && strings.HasPrefix(partMediaType, "text/") {
			firstText = content
		}
	}

	return firstText, nil
}

func readDecodedBody(body io.Reader, transferEncoding string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(transferEncoding)) {
	case "quoted-printable":
		data, err := io.ReadAll(quotedprintable.NewReader(body))
		if err != nil {
			return "", fmt.Errorf("decode quoted-printable email body: %w", err)
		}
		return string(data), nil
	case "base64":
		data, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, body))
		if err != nil {
			return "", fmt.Errorf("decode base64 email body: %w", err)
		}
		return string(data), nil
	default:
		data, err := io.ReadAll(body)
		if err != nil {
			return "", fmt.Errorf("read email body: %w", err)
		}
		return string(data), nil
	}
}

func decodeHeader(value string) string {
	decoded, err := new(mime.WordDecoder).DecodeHeader(value)
	if err != nil {
		return value
	}
	return decoded
}

func htmlToText(value string) string {
	doc, err := html.Parse(strings.NewReader(value))
	if err != nil {
		return normalizeText(value)
	}

	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				if b.Len() > 0 {
					b.WriteByte(' ')
				}
				b.WriteString(text)
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)

	return normalizeText(b.String())
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
