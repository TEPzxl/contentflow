package article

import "time"

type listArticlesHTTPResp struct {
	Articles []articleHTTPResp `json:"articles"`
	Total    int64             `json:"total"`
	Limit    int               `json:"limit"`
	Offset   int               `json:"offset"`
}

type articleHTTPWrapper struct {
	Article articleHTTPResp `json:"article"`
}

type articleHTTPResp struct {
	ID          int64      `json:"id"`
	SourceID    int64      `json:"source_id"`
	SourceType  string     `json:"source_type"`
	ExternalID  *string    `json:"external_id,omitempty"`
	Title       string     `json:"title"`
	URL         *string    `json:"url,omitempty"`
	OriginalURL *string    `json:"original_url,omitempty"`
	Author      string     `json:"author"`
	Summary     string     `json:"summary"`
	Content     string     `json:"content,omitempty"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	IsRead      bool       `json:"is_read"`
	IsSaved     bool       `json:"is_saved"`
	ReadAt      *time.Time `json:"read_at,omitempty"`
	SavedAt     *time.Time `json:"saved_at,omitempty"`
}

type updateReadHTTPReq struct {
	IsRead *bool `json:"is_read"`
}

type updateSaveHTTPReq struct {
	IsSaved *bool `json:"is_saved"`
}

func toArticleHTTPRespList(articles []ArticleDTO) []articleHTTPResp {
	items := make([]articleHTTPResp, 0, len(articles))
	for _, article := range articles {
		items = append(items, toArticleHTTPResp(article))
	}
	return items
}

func toArticleHTTPResp(article ArticleDTO) articleHTTPResp {
	return articleHTTPResp{
		ID:          article.ID,
		SourceID:    article.SourceID,
		SourceType:  article.SourceType,
		ExternalID:  article.ExternalID,
		Title:       article.Title,
		URL:         article.URL,
		OriginalURL: article.OriginalURL,
		Author:      article.Author,
		Summary:     article.Summary,
		Content:     article.Content,
		PublishedAt: article.PublishedAt,
		CreatedAt:   article.CreatedAt,
		UpdatedAt:   article.UpdatedAt,
		IsRead:      article.IsRead,
		IsSaved:     article.IsSaved,
		ReadAt:      article.ReadAt,
		SavedAt:     article.SavedAt,
	}
}
