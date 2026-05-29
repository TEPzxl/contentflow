package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository interface {
	EnqueueSummary(ctx context.Context, params EnqueueSummaryParams) (*ArticleSummary, error)
	FindSummary(ctx context.Context, userID, articleID int64, promptVersion string) (*ArticleSummary, error)
	ClaimNextSummary(ctx context.Context, now time.Time, maxAttempts int) (*ArticleSummary, error)
	CompleteSummary(ctx context.Context, id int64, summary string, now time.Time) (*ArticleSummary, error)
	FailSummary(ctx context.Context, id int64, errMessage string, nextAttemptAt time.Time, now time.Time) (*ArticleSummary, error)
	UpsertEmbedding(ctx context.Context, params UpsertEmbeddingParams) (*EmbeddingRecord, error)
	FindEmbedding(ctx context.Context, userID, articleID int64, model, version string) (*EmbeddingRecord, error)
	ListEmbeddingsByUser(ctx context.Context, userID int64, model, version string, limit int) ([]EmbeddingRecord, error)
	UpsertDigest(ctx context.Context, params UpsertDigestParams) (*DigestRecord, error)
	FindDigest(ctx context.Context, userID int64, digestDate time.Time, promptVersion string) (*DigestRecord, error)
	FindAISettings(ctx context.Context, userID int64) (*UserAISettingsRecord, error)
	UpsertAISettings(ctx context.Context, params UpsertAISettingsParams) (*UserAISettingsRecord, error)
}

type EnqueueSummaryParams struct {
	UserID        int64
	ArticleID     int64
	Model         string
	PromptVersion string
	Now           time.Time
	Regenerate    bool
}

type UpsertEmbeddingParams struct {
	UserID      int64
	ArticleID   int64
	Model       string
	Version     string
	Dimensions  int
	Embedding   []float64
	ContentHash string
	Now         time.Time
}

type UpsertDigestParams struct {
	UserID        int64
	DigestDate    time.Time
	Model         string
	PromptVersion string
	Summary       string
	ArticleIDs    []int64
	Now           time.Time
}

type UpsertAISettingsParams struct {
	UserID           int64
	Provider         string
	BaseURL          string
	Model            string
	EmbeddingModel   string
	APIKeyCiphertext []byte
	APIKeyNonce      []byte
	Now              time.Time
}

var (
	ErrSummaryNotFound                 = errors.New("summary not found")
	ErrEmbeddingNotFound               = errors.New("embedding not found")
	ErrDigestNotFound                  = errors.New("digest not found")
	ErrAISettingsNotFound              = errors.New("ai settings not found")
	ErrAISettingsEncryptionKeyRequired = errors.New("ai settings encryption key required")
	ErrInvalidAIProvider               = errors.New("invalid ai provider")
	ErrInvalidAIBaseURL                = errors.New("invalid ai base url")
	ErrInvalidAIModel                  = errors.New("invalid ai model")
	ErrNoSummaryJob                    = errors.New("no summary job")
)

type GormRepository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &GormRepository{db: db}
}

func (r *GormRepository) EnqueueSummary(ctx context.Context, params EnqueueSummaryParams) (*ArticleSummary, error) {
	summary := ArticleSummary{
		UserID:        params.UserID,
		ArticleID:     params.ArticleID,
		Model:         params.Model,
		PromptVersion: params.PromptVersion,
		Status:        StatusPending,
		NextAttemptAt: params.Now,
		CreatedAt:     params.Now,
		UpdatedAt:     params.Now,
	}

	assignments := clause.Assignments(map[string]any{
		"model":           params.Model,
		"status":          StatusPending,
		"next_attempt_at": params.Now,
		"error_message":   "",
		"updated_at":      params.Now,
	})
	if params.Regenerate {
		assignments = append(assignments,
			clause.Assignment{Column: clause.Column{Name: "summary"}, Value: ""},
			clause.Assignment{Column: clause.Column{Name: "attempts"}, Value: 0},
		)
	}

	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "article_id"}, {Name: "prompt_version"}},
			DoUpdates: assignments,
		}).
		Create(&summary).Error; err != nil {
		return nil, fmt.Errorf("enqueue article summary: %w", err)
	}

	return r.FindSummary(ctx, params.UserID, params.ArticleID, params.PromptVersion)
}

func (r *GormRepository) FindSummary(ctx context.Context, userID, articleID int64, promptVersion string) (*ArticleSummary, error) {
	var summary ArticleSummary
	err := r.db.WithContext(ctx).
		Table("article_summaries AS s").
		Select("s.*").
		Joins("JOIN articles AS a ON a.id = s.article_id").
		Joins("JOIN sources AS src ON src.id = a.source_id AND src.user_id = ?", userID).
		Where("s.user_id = ? AND s.article_id = ? AND s.prompt_version = ?", userID, articleID, promptVersion).
		Where("src.deleted_at IS NULL").
		First(&summary).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrSummaryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find article summary: %w", err)
	}
	return &summary, nil
}

func (r *GormRepository) ClaimNextSummary(ctx context.Context, now time.Time, maxAttempts int) (*ArticleSummary, error) {
	var summary ArticleSummary
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status IN ? AND next_attempt_at <= ? AND attempts < ?", []string{StatusPending, StatusFailed}, now, maxAttempts).
			Order("next_attempt_at ASC").
			First(&summary).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNoSummaryJob
		}
		if err != nil {
			return fmt.Errorf("claim article summary: %w", err)
		}

		return tx.Model(&ArticleSummary{}).
			Where("id = ?", summary.ID).
			Updates(map[string]any{
				"status":     StatusProcessing,
				"attempts":   gorm.Expr("attempts + 1"),
				"updated_at": now,
			}).Error
	})
	if errors.Is(err, ErrNoSummaryJob) {
		return nil, ErrNoSummaryJob
	}
	if err != nil {
		return nil, err
	}
	return r.FindSummary(ctx, summary.UserID, summary.ArticleID, summary.PromptVersion)
}

func (r *GormRepository) CompleteSummary(ctx context.Context, id int64, summaryText string, now time.Time) (*ArticleSummary, error) {
	var summary ArticleSummary
	if err := r.db.WithContext(ctx).Model(&ArticleSummary{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"summary":       summaryText,
			"status":        StatusSucceeded,
			"error_message": "",
			"updated_at":    now,
		}).Error; err != nil {
		return nil, fmt.Errorf("complete article summary: %w", err)
	}
	if err := r.db.WithContext(ctx).First(&summary, id).Error; err != nil {
		return nil, fmt.Errorf("reload article summary: %w", err)
	}
	return &summary, nil
}

func (r *GormRepository) FailSummary(ctx context.Context, id int64, errMessage string, nextAttemptAt time.Time, now time.Time) (*ArticleSummary, error) {
	var summary ArticleSummary
	if err := r.db.WithContext(ctx).Model(&ArticleSummary{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":          StatusFailed,
			"error_message":   errMessage,
			"next_attempt_at": nextAttemptAt,
			"updated_at":      now,
		}).Error; err != nil {
		return nil, fmt.Errorf("fail article summary: %w", err)
	}
	if err := r.db.WithContext(ctx).First(&summary, id).Error; err != nil {
		return nil, fmt.Errorf("reload failed article summary: %w", err)
	}
	return &summary, nil
}

func (r *GormRepository) UpsertEmbedding(ctx context.Context, params UpsertEmbeddingParams) (*EmbeddingRecord, error) {
	payload, err := json.Marshal(params.Embedding)
	if err != nil {
		return nil, fmt.Errorf("marshal embedding: %w", err)
	}

	model := ArticleEmbedding{
		UserID:        params.UserID,
		ArticleID:     params.ArticleID,
		Model:         params.Model,
		Version:       params.Version,
		Dimensions:    params.Dimensions,
		EmbeddingJSON: datatypes.JSON(payload),
		ContentHash:   params.ContentHash,
		CreatedAt:     params.Now,
		UpdatedAt:     params.Now,
	}
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "article_id"}, {Name: "model"}, {Name: "version"}},
			DoUpdates: clause.Assignments(map[string]any{
				"dimensions":     params.Dimensions,
				"embedding_json": datatypes.JSON(payload),
				"content_hash":   params.ContentHash,
				"updated_at":     params.Now,
			}),
		}).
		Create(&model).Error; err != nil {
		return nil, fmt.Errorf("upsert article embedding: %w", err)
	}
	return r.FindEmbedding(ctx, params.UserID, params.ArticleID, params.Model, params.Version)
}

func (r *GormRepository) FindEmbedding(ctx context.Context, userID, articleID int64, modelName, version string) (*EmbeddingRecord, error) {
	var model ArticleEmbedding
	err := r.db.WithContext(ctx).
		Table("article_embeddings AS e").
		Select("e.*").
		Joins("JOIN articles AS a ON a.id = e.article_id").
		Joins("JOIN sources AS src ON src.id = a.source_id AND src.user_id = ?", userID).
		Where("e.user_id = ? AND e.article_id = ? AND e.model = ? AND e.version = ?", userID, articleID, modelName, version).
		Where("src.deleted_at IS NULL").
		First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrEmbeddingNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find article embedding: %w", err)
	}
	return embeddingModelToRecord(model)
}

func (r *GormRepository) ListEmbeddingsByUser(ctx context.Context, userID int64, modelName, version string, limit int) ([]EmbeddingRecord, error) {
	var models []ArticleEmbedding
	query := r.db.WithContext(ctx).
		Table("article_embeddings AS e").
		Select("e.*").
		Joins("JOIN articles AS a ON a.id = e.article_id").
		Joins("JOIN sources AS src ON src.id = a.source_id AND src.user_id = ?", userID).
		Where("e.user_id = ? AND e.model = ? AND e.version = ?", userID, modelName, version).
		Where("src.deleted_at IS NULL").
		Order("e.updated_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list article embeddings: %w", err)
	}
	records := make([]EmbeddingRecord, 0, len(models))
	for _, model := range models {
		record, err := embeddingModelToRecord(model)
		if err != nil {
			return nil, err
		}
		records = append(records, *record)
	}
	return records, nil
}

func (r *GormRepository) UpsertDigest(ctx context.Context, params UpsertDigestParams) (*DigestRecord, error) {
	ids, err := json.Marshal(params.ArticleIDs)
	if err != nil {
		return nil, fmt.Errorf("marshal digest article ids: %w", err)
	}

	model := DailyDigest{
		UserID:        params.UserID,
		DigestDate:    normalizeDate(params.DigestDate),
		Model:         params.Model,
		PromptVersion: params.PromptVersion,
		Summary:       params.Summary,
		ArticleIDs:    datatypes.JSON(ids),
		Status:        StatusSucceeded,
		CreatedAt:     params.Now,
		UpdatedAt:     params.Now,
	}
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}, {Name: "digest_date"}, {Name: "prompt_version"}},
			DoUpdates: clause.Assignments(map[string]any{
				"model":         params.Model,
				"summary":       params.Summary,
				"article_ids":   datatypes.JSON(ids),
				"status":        StatusSucceeded,
				"error_message": "",
				"updated_at":    params.Now,
			}),
		}).
		Create(&model).Error; err != nil {
		return nil, fmt.Errorf("upsert daily digest: %w", err)
	}
	return r.FindDigest(ctx, params.UserID, params.DigestDate, params.PromptVersion)
}

func (r *GormRepository) FindDigest(ctx context.Context, userID int64, digestDate time.Time, promptVersion string) (*DigestRecord, error) {
	var model DailyDigest
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND digest_date = ? AND prompt_version = ?", userID, normalizeDate(digestDate), promptVersion).
		First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrDigestNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find daily digest: %w", err)
	}
	return digestModelToRecord(model)
}

func (r *GormRepository) FindAISettings(ctx context.Context, userID int64) (*UserAISettingsRecord, error) {
	var model UserAISettings
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAISettingsNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find ai settings: %w", err)
	}
	return aiSettingsModelToRecord(model), nil
}

func (r *GormRepository) UpsertAISettings(ctx context.Context, params UpsertAISettingsParams) (*UserAISettingsRecord, error) {
	model := UserAISettings{
		UserID:           params.UserID,
		Provider:         params.Provider,
		BaseURL:          params.BaseURL,
		Model:            params.Model,
		EmbeddingModel:   params.EmbeddingModel,
		APIKeyCiphertext: params.APIKeyCiphertext,
		APIKeyNonce:      params.APIKeyNonce,
		CreatedAt:        params.Now,
		UpdatedAt:        params.Now,
	}
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"provider":           params.Provider,
				"base_url":           params.BaseURL,
				"model":              params.Model,
				"embedding_model":    params.EmbeddingModel,
				"api_key_ciphertext": params.APIKeyCiphertext,
				"api_key_nonce":      params.APIKeyNonce,
				"updated_at":         params.Now,
			}),
		}).
		Create(&model).Error; err != nil {
		return nil, fmt.Errorf("upsert ai settings: %w", err)
	}
	return r.FindAISettings(ctx, params.UserID)
}

func embeddingModelToRecord(model ArticleEmbedding) (*EmbeddingRecord, error) {
	var vector []float64
	if err := json.Unmarshal(model.EmbeddingJSON, &vector); err != nil {
		return nil, fmt.Errorf("unmarshal embedding: %w", err)
	}
	return &EmbeddingRecord{
		ID:          model.ID,
		UserID:      model.UserID,
		ArticleID:   model.ArticleID,
		Model:       model.Model,
		Version:     model.Version,
		Dimensions:  model.Dimensions,
		Embedding:   vector,
		ContentHash: model.ContentHash,
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
	}, nil
}

func digestModelToRecord(model DailyDigest) (*DigestRecord, error) {
	var ids []int64
	if err := json.Unmarshal(model.ArticleIDs, &ids); err != nil {
		return nil, fmt.Errorf("unmarshal digest article ids: %w", err)
	}
	return &DigestRecord{
		ID:            model.ID,
		UserID:        model.UserID,
		DigestDate:    model.DigestDate,
		Model:         model.Model,
		PromptVersion: model.PromptVersion,
		Summary:       model.Summary,
		ArticleIDs:    ids,
		Status:        model.Status,
		ErrorMessage:  model.ErrorMessage,
		CreatedAt:     model.CreatedAt,
		UpdatedAt:     model.UpdatedAt,
	}, nil
}

func aiSettingsModelToRecord(model UserAISettings) *UserAISettingsRecord {
	return &UserAISettingsRecord{
		ID:               model.ID,
		UserID:           model.UserID,
		Provider:         model.Provider,
		BaseURL:          model.BaseURL,
		Model:            model.Model,
		EmbeddingModel:   model.EmbeddingModel,
		APIKeyCiphertext: model.APIKeyCiphertext,
		APIKeyNonce:      model.APIKeyNonce,
		CreatedAt:        model.CreatedAt,
		UpdatedAt:        model.UpdatedAt,
	}
}

func normalizeDate(value time.Time) time.Time {
	year, month, day := value.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}
