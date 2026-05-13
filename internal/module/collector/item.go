package collector

import "time"

type CollectedItem struct {
	SourceID    int64
	SourceType  string
	ExternalID  *string
	Title       string
	URL         *string
	OriginalURL *string
	Author      string
	Summary     string
	Content     string
	ContentHash string
	PublishedAt *time.Time
}
