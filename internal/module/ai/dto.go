package ai

import "time"

type SummaryDTO struct {
	ID            int64     `json:"id"`
	ArticleID     int64     `json:"article_id"`
	Model         string    `json:"model"`
	PromptVersion string    `json:"prompt_version"`
	Summary       string    `json:"summary"`
	Status        string    `json:"status"`
	Attempts      int       `json:"attempts"`
	ErrorMessage  string    `json:"error_message"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type EmbeddingDTO struct {
	ID          int64     `json:"id"`
	ArticleID   int64     `json:"article_id"`
	Model       string    `json:"model"`
	Version     string    `json:"version"`
	Dimensions  int       `json:"dimensions"`
	ContentHash string    `json:"content_hash"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type SimilarArticleDTO struct {
	ArticleID int64   `json:"article_id"`
	Title     string  `json:"title"`
	Summary   string  `json:"summary"`
	URL       *string `json:"url"`
	Score     float64 `json:"score"`
}

type DigestDTO struct {
	ID            int64     `json:"id"`
	DigestDate    string    `json:"digest_date"`
	Model         string    `json:"model"`
	PromptVersion string    `json:"prompt_version"`
	Summary       string    `json:"summary"`
	ArticleIDs    []int64   `json:"article_ids"`
	Status        string    `json:"status"`
	ErrorMessage  string    `json:"error_message"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type CitationDTO struct {
	ArticleID int64   `json:"article_id"`
	Title     string  `json:"title"`
	URL       *string `json:"url"`
	Snippet   string  `json:"snippet"`
}

type RAGAnswerDTO struct {
	Model         string        `json:"model"`
	PromptVersion string        `json:"prompt_version"`
	Answer        string        `json:"answer"`
	Citations     []CitationDTO `json:"citations"`
}

type RequestSummaryRequest struct {
	UserID     int64
	ArticleID  int64
	Regenerate bool
}

type GetSummaryRequest struct {
	UserID    int64
	ArticleID int64
}

type GenerateEmbeddingRequest struct {
	UserID    int64
	ArticleID int64
}

type SimilarArticlesRequest struct {
	UserID    int64
	ArticleID int64
	Limit     int
}

type GenerateDigestRequest struct {
	UserID int64
	Date   time.Time
}

type GetDigestRequest struct {
	UserID int64
	Date   time.Time
}

type RAGSearchRequest struct {
	UserID int64
	Query  string
	Limit  int
}
