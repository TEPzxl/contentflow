package article

import "time"

type Article struct {
	ID          int64      `gorm:"column:id;primaryKey"`
	SourceID    int64      `gorm:"column:source_id;not null;index"`
	SourceType  string     `gorm:"column:source_type;type:varchar(50);not null"`
	ExternalID  *string    `gorm:"column:external_id;type:text"`
	Title       string     `gorm:"column:title;type:text;not null"`
	URL         *string    `gorm:"column:url;type:text"`
	OriginalURL *string    `gorm:"column:original_url;type:text"`
	Author      string     `gorm:"column:author;type:text;not null;default:''"`
	Summary     string     `gorm:"column:summary;type:text;not null;default:''"`
	Content     string     `gorm:"column:content;type:text;not null;default:''"`
	ContentHash string     `gorm:"column:content_hash;type:char(64);not null;index"`
	PublishedAt *time.Time `gorm:"column:published_at;index"`
	CreatedAt   time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;not null"`
}

func (Article) TableName() string {
	return "articles"
}

type ArticleState struct {
	ID        int64      `gorm:"column:id;primaryKey"`
	UserID    int64      `gorm:"column:user_id;not null;index"`
	ArticleID int64      `gorm:"column:article_id;not null;index"`
	IsRead    bool       `gorm:"column:is_read;not null;default:false"`
	IsSaved   bool       `gorm:"column:is_saved;not null;default:false"`
	ReadAt    *time.Time `gorm:"column:read_at"`
	SavedAt   *time.Time `gorm:"column:saved_at"`
	CreatedAt time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt time.Time  `gorm:"column:updated_at;not null"`
}

func (ArticleState) TableName() string {
	return "article_states"
}
