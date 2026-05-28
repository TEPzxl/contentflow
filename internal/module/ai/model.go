package ai

import (
	"time"

	"gorm.io/datatypes"
)

const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusSucceeded  = "succeeded"
	StatusFailed     = "failed"

	DefaultSummaryModel       = "contentflow-extractive-v1"
	DefaultSummaryPrompt      = "summary-v1"
	DefaultEmbeddingModel     = "contentflow-hash-embedding-v1"
	DefaultEmbeddingVersion   = "embedding-v1"
	DefaultEmbeddingDimension = 16
	DefaultDigestPrompt       = "digest-v1"
	DefaultRAGPrompt          = "rag-v1"
)

type ArticleSummary struct {
	ID            int64     `gorm:"column:id;primaryKey"`
	UserID        int64     `gorm:"column:user_id;not null;index"`
	ArticleID     int64     `gorm:"column:article_id;not null;index"`
	Model         string    `gorm:"column:model;type:varchar(120);not null"`
	PromptVersion string    `gorm:"column:prompt_version;type:varchar(80);not null"`
	Summary       string    `gorm:"column:summary;type:text;not null;default:''"`
	Status        string    `gorm:"column:status;type:varchar(50);not null"`
	Attempts      int       `gorm:"column:attempts;not null;default:0"`
	NextAttemptAt time.Time `gorm:"column:next_attempt_at;not null"`
	ErrorMessage  string    `gorm:"column:error_message;type:text;not null;default:''"`
	CreatedAt     time.Time `gorm:"column:created_at;not null"`
	UpdatedAt     time.Time `gorm:"column:updated_at;not null"`
}

func (ArticleSummary) TableName() string {
	return "article_summaries"
}

type ArticleEmbedding struct {
	ID            int64          `gorm:"column:id;primaryKey"`
	UserID        int64          `gorm:"column:user_id;not null;index"`
	ArticleID     int64          `gorm:"column:article_id;not null;index"`
	Model         string         `gorm:"column:model;type:varchar(120);not null"`
	Version       string         `gorm:"column:version;type:varchar(80);not null"`
	Dimensions    int            `gorm:"column:dimensions;not null"`
	EmbeddingJSON datatypes.JSON `gorm:"column:embedding_json;type:jsonb;not null"`
	ContentHash   string         `gorm:"column:content_hash;type:char(64);not null;index"`
	CreatedAt     time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt     time.Time      `gorm:"column:updated_at;not null"`
}

func (ArticleEmbedding) TableName() string {
	return "article_embeddings"
}

type DailyDigest struct {
	ID            int64          `gorm:"column:id;primaryKey"`
	UserID        int64          `gorm:"column:user_id;not null;index"`
	DigestDate    time.Time      `gorm:"column:digest_date;type:date;not null"`
	Model         string         `gorm:"column:model;type:varchar(120);not null"`
	PromptVersion string         `gorm:"column:prompt_version;type:varchar(80);not null"`
	Summary       string         `gorm:"column:summary;type:text;not null;default:''"`
	ArticleIDs    datatypes.JSON `gorm:"column:article_ids;type:jsonb;not null"`
	Status        string         `gorm:"column:status;type:varchar(50);not null"`
	ErrorMessage  string         `gorm:"column:error_message;type:text;not null;default:''"`
	CreatedAt     time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt     time.Time      `gorm:"column:updated_at;not null"`
}

func (DailyDigest) TableName() string {
	return "daily_digests"
}

type UserAISettings struct {
	ID               int64     `gorm:"column:id;primaryKey"`
	UserID           int64     `gorm:"column:user_id;not null;uniqueIndex"`
	Provider         string    `gorm:"column:provider;type:varchar(50);not null"`
	BaseURL          string    `gorm:"column:base_url;type:text;not null;default:''"`
	Model            string    `gorm:"column:model;type:varchar(120);not null;default:''"`
	EmbeddingModel   string    `gorm:"column:embedding_model;type:varchar(120);not null;default:''"`
	APIKeyCiphertext []byte    `gorm:"column:api_key_ciphertext;type:bytea"`
	APIKeyNonce      []byte    `gorm:"column:api_key_nonce;type:bytea"`
	CreatedAt        time.Time `gorm:"column:created_at;not null"`
	UpdatedAt        time.Time `gorm:"column:updated_at;not null"`
}

func (UserAISettings) TableName() string {
	return "user_ai_settings"
}

type EmbeddingRecord struct {
	ID          int64
	UserID      int64
	ArticleID   int64
	Model       string
	Version     string
	Dimensions  int
	Embedding   []float64
	ContentHash string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type DigestRecord struct {
	ID            int64
	UserID        int64
	DigestDate    time.Time
	Model         string
	PromptVersion string
	Summary       string
	ArticleIDs    []int64
	Status        string
	ErrorMessage  string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type UserAISettingsRecord struct {
	ID               int64
	UserID           int64
	Provider         string
	BaseURL          string
	Model            string
	EmbeddingModel   string
	APIKeyCiphertext []byte
	APIKeyNonce      []byte
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
