package article

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository interface {
	CreateIfNotExists(ctx context.Context, article *Article) (bool, error)
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
