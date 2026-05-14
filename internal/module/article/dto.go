package article

import "time"

type ListArticlesRequest struct {
	UserID   int64
	SourceID int64
	Query    string
	IsRead   *bool
	IsSaved  *bool
	Limit    int
	Offset   int
}

type ListArticlesResponse struct {
	Articles []ArticleDTO
	Total    int64
	Limit    int
	Offset   int
}

type GetArticleRequest struct {
	UserID    int64
	ArticleID int64
}

type GetArticleResponse struct {
	Article ArticleDTO
}

type UpdateArticleStateRequest struct {
	UserID    int64
	ArticleID int64
	IsRead    *bool
	IsSaved   *bool
}

type UpdateArticleStateResponse struct {
	Article ArticleDTO
}

type ArticleDTO struct {
	ID          int64
	SourceID    int64
	SourceType  string
	ExternalID  *string
	Title       string
	URL         *string
	OriginalURL *string
	Author      string
	Summary     string
	Content     string
	PublishedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
	IsRead      bool
	IsSaved     bool
	ReadAt      *time.Time
	SavedAt     *time.Time
}
