package article

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository interface {
	CreateIfNotExists(ctx context.Context, article *Article) (bool, error)
	ListByUser(ctx context.Context, params ListArticlesParams) ([]ArticleWithState, int64, error)
	FindByUserAndID(ctx context.Context, userID, articleID int64) (ArticleWithState, error)
	UpsertState(ctx context.Context, params UpsertArticleStateParams) (ArticleWithState, error)
}

var ErrArticleNotFound = errors.New("article not found")

type ListArticlesParams struct {
	UserID   int64
	SourceID int64
	Query    string
	IsRead   *bool
	IsSaved  *bool
	Limit    int
	Offset   int
}

type UpsertArticleStateParams struct {
	UserID    int64
	ArticleID int64
	IsRead    *bool
	IsSaved   *bool
	Now       time.Time
}

type ArticleWithState struct {
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

type GormRepository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &GormRepository{db: db}
}

func (r *GormRepository) CreateIfNotExists(ctx context.Context, article *Article) (bool, error) {
	result := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			DoNothing: true,
		}).
		Create(article)

	if result.Error != nil {
		return false, fmt.Errorf("create article if not exists: %w", result.Error)
	}
	return result.RowsAffected > 0, nil

}

func (r *GormRepository) ListByUser(ctx context.Context, params ListArticlesParams) ([]ArticleWithState, int64, error) {
	query := r.articleQuery(ctx, params.UserID).
		Where("src.deleted_at IS NULL")

	query = applyArticleFilters(query, params)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count articles by user: %w", err)
	}

	var rows []ArticleWithState
	if err := query.
		Order("a.published_at DESC NULLS LAST").
		Order("a.created_at DESC").
		Limit(params.Limit).
		Offset(params.Offset).
		Scan(&rows).
		Error; err != nil {
		return nil, 0, fmt.Errorf("list articles by user: %w", err)
	}

	return rows, total, nil
}

func (r *GormRepository) FindByUserAndID(ctx context.Context, userID, articleID int64) (ArticleWithState, error) {
	var row ArticleWithState
	err := r.articleQuery(ctx, userID).
		Where("a.id = ?", articleID).
		Where("src.deleted_at IS NULL").
		First(&row).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ArticleWithState{}, ErrArticleNotFound
	}
	if err != nil {
		return ArticleWithState{}, fmt.Errorf("find article by user and id: %w", err)
	}

	return row, nil
}

func (r *GormRepository) UpsertState(ctx context.Context, params UpsertArticleStateParams) (ArticleWithState, error) {
	_, err := r.FindByUserAndID(ctx, params.UserID, params.ArticleID)
	if err != nil {
		return ArticleWithState{}, err
	}

	state := ArticleState{
		UserID:    params.UserID,
		ArticleID: params.ArticleID,
		CreatedAt: params.Now,
		UpdatedAt: params.Now,
	}

	assignments := clause.Assignments(map[string]any{
		"updated_at": params.Now,
	})

	if params.IsRead != nil {
		state.IsRead = *params.IsRead
		assignments = append(assignments, clause.Assignment{Column: clause.Column{Name: "is_read"}, Value: *params.IsRead})
		if *params.IsRead {
			state.ReadAt = &params.Now
			assignments = append(assignments, clause.Assignment{Column: clause.Column{Name: "read_at"}, Value: params.Now})
		} else {
			assignments = append(assignments, clause.Assignment{Column: clause.Column{Name: "read_at"}, Value: nil})
		}
	}

	if params.IsSaved != nil {
		state.IsSaved = *params.IsSaved
		assignments = append(assignments, clause.Assignment{Column: clause.Column{Name: "is_saved"}, Value: *params.IsSaved})
		if *params.IsSaved {
			state.SavedAt = &params.Now
			assignments = append(assignments, clause.Assignment{Column: clause.Column{Name: "saved_at"}, Value: params.Now})
		} else {
			assignments = append(assignments, clause.Assignment{Column: clause.Column{Name: "saved_at"}, Value: nil})
		}
	}

	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "article_id"}},
			DoUpdates: assignments,
		}).
		Create(&state).
		Error; err != nil {
		return ArticleWithState{}, fmt.Errorf("upsert article state: %w", err)
	}

	return r.FindByUserAndID(ctx, params.UserID, params.ArticleID)
}

func (r *GormRepository) articleQuery(ctx context.Context, userID int64) *gorm.DB {
	return r.db.WithContext(ctx).
		Table("articles AS a").
		Select(`
			a.id,
			a.source_id,
			a.source_type,
			a.external_id,
			a.title,
			a.url,
			a.original_url,
			a.author,
			a.summary,
			a.content,
			a.published_at,
			a.created_at,
			a.updated_at,
			COALESCE(st.is_read, false) AS is_read,
			COALESCE(st.is_saved, false) AS is_saved,
			st.read_at,
			st.saved_at
		`).
		Joins("JOIN sources AS src ON src.id = a.source_id AND src.user_id = ?", userID).
		Joins("LEFT JOIN article_states AS st ON st.article_id = a.id AND st.user_id = ?", userID)
}

func applyArticleFilters(query *gorm.DB, params ListArticlesParams) *gorm.DB {
	if params.SourceID > 0 {
		query = query.Where("a.source_id = ?", params.SourceID)
	}

	if params.Query != "" {
		like := "%" + strings.TrimSpace(params.Query) + "%"
		query = query.Where("(a.title ILIKE ? OR a.summary ILIKE ? OR a.content ILIKE ?)", like, like, like)
	}

	if params.IsRead != nil {
		query = query.Where("COALESCE(st.is_read, false) = ?", *params.IsRead)
	}

	if params.IsSaved != nil {
		query = query.Where("COALESCE(st.is_saved, false) = ?", *params.IsSaved)
	}

	return query
}
