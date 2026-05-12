package user

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

var ErrUserNotFound = errors.New("user not found")

type Repository interface {
	Create(ctx context.Context, u *User) error
	FindByID(ctx context.Context, id int64) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
}

type GormRepository struct {
	db *gorm.DB
}

func NewGormRepository(db *gorm.DB) *GormRepository {
	return &GormRepository{db: db}
}

func (r *GormRepository) Create(ctx context.Context, u *User) error {
	if err := gorm.G[User](r.db).Create(ctx, u); err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (r *GormRepository) FindByID(ctx context.Context, id int64) (*User, error) {
	u, err := gorm.G[User](r.db).Where("id = ?", id).First(ctx)

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find user by id: %w", err)
	}

	return &u, nil
}

func (r *GormRepository) FindByEmail(ctx context.Context, email string) (*User, error) {
	u, err := gorm.G[User](r.db).Where("email = ?", email).First(ctx)

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find user by email: %w", err)
	}

	return &u, nil
}
