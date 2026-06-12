package source

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tepzxl/contentflow/internal/storage/dbtx"
	"gorm.io/gorm"
)

var (
	ErrSourceNotFound      = errors.New("source not found")
	ErrSourceURLDuplicated = errors.New("source URL duplicated")
)

type ListParams struct {
	UserID int64
	Type   string
	Limit  int
	Offset int
}

type ActiveSourceForCollection struct {
	ID            int64      `gorm:"column:id"`
	UserID        int64      `gorm:"column:user_id"`
	Type          string     `gorm:"column:type"`
	LastFetchedAt *time.Time `gorm:"column:last_fetched_at"`
}

type Repository interface {
	Create(ctx context.Context, s *Source) error
	FindByUserIDAndID(ctx context.Context, userID, id int64) (*Source, error)
	ListByUserID(ctx context.Context, params ListParams) ([]Source, int64, error)
	ListActiveForCollection(ctx context.Context, limit int) ([]ActiveSourceForCollection, error)
	Update(ctx context.Context, s *Source) error
	SoftDelete(ctx context.Context, userID, id int64, deletedAt time.Time) error
}

var _ Repository = (*GormRepository)(nil)

type GormRepository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &GormRepository{db: db}
}

func (r *GormRepository) Create(ctx context.Context, s *Source) error {
	if err := gorm.G[Source](dbtx.FromContext(ctx, r.db)).Create(ctx, s); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return ErrSourceURLDuplicated
		}
		return fmt.Errorf("create source: %w", err)
	}
	return nil
}

func (r *GormRepository) FindByUserIDAndID(ctx context.Context, userID, id int64) (*Source, error) {
	s, err := gorm.G[Source](dbtx.FromContext(ctx, r.db)).
		Where("user_id = ?", userID).
		Where("id = ?", id).
		Where("deleted_at IS NULL").
		First(ctx)

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrSourceNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find source by user id and id: %w", err)
	}

	return &s, nil
}

func (r *GormRepository) ListByUserID(ctx context.Context, params ListParams) ([]Source, int64, error) {
	query := dbtx.FromContext(ctx, r.db).WithContext(ctx).
		Model(&Source{}).
		Where("user_id = ?", params.UserID).
		Where("deleted_at IS NULL")

	if params.Type != "" {
		query = query.Where("type = ?", params.Type)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count sources by user id: %w", err)
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}

	if limit > 100 {
		limit = 100
	}

	offset := params.Offset
	if offset < 0 {
		offset = 0
	}

	var sources []Source
	if err := query.
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&sources).
		Error; err != nil {
		return nil, 0, fmt.Errorf("list sources by user id: %w", err)
	}

	return sources, total, nil
}

func (r *GormRepository) ListActiveForCollection(ctx context.Context, limit int) ([]ActiveSourceForCollection, error) {
	if limit <= 0 {
		limit = 100
	}

	var sources []ActiveSourceForCollection
	if err := dbtx.FromContext(ctx, r.db).WithContext(ctx).
		Model(&Source{}).
		Select("id", "user_id", "type", "last_fetched_at").
		Where("is_active = ?", true).
		Where("deleted_at IS NULL").
		Order("last_fetched_at ASC NULLS FIRST").
		Order("created_at ASC").
		Limit(limit).
		Find(&sources).
		Error; err != nil {
		return nil, fmt.Errorf("list active sources for collection: %w", err)
	}

	return sources, nil
}

func (r *GormRepository) Update(ctx context.Context, s *Source) error {
	updates := map[string]any{
		"name":               s.Name,
		"url":                s.URL,
		"config_json":        s.ConfigJSON,
		"is_active":          s.IsActive,
		"last_fetched_at":    s.LastFetchedAt,
		"last_fetch_status":  s.LastFetchStatus,
		"last_fetch_message": s.LastFetchMessage,
		"updated_at":         s.UpdatedAt,
	}

	result := dbtx.FromContext(ctx, r.db).WithContext(ctx).
		Model(&Source{}).
		Where("user_id = ?", s.UserID).
		Where("id = ?", s.ID).
		Where("deleted_at IS NULL").
		Updates(updates)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrDuplicatedKey) {
			return ErrSourceURLDuplicated
		}
		return fmt.Errorf("update source: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return ErrSourceNotFound
	}

	return nil
}

func (r *GormRepository) SoftDelete(ctx context.Context, userID, id int64, deletedAt time.Time) error {
	result := dbtx.FromContext(ctx, r.db).WithContext(ctx).
		Model(&Source{}).
		Where("user_id = ?", userID).
		Where("id = ?", id).
		Where("deleted_at IS NULL").
		Updates(map[string]any{
			"deleted_at": deletedAt,
			"updated_at": deletedAt,
			"is_active":  false,
		})

	if result.Error != nil {
		return fmt.Errorf("soft delete source: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return ErrSourceNotFound
	}

	return nil
}
