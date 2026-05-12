package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrEmailAlreadyExists = errors.New("email already exists")
)

type Repository interface {
	Create(ctx context.Context, u *User) error
	FindByID(ctx context.Context, id int64) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
}

var _ Repository = (*GormRepository)(nil)

type GormRepository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *GormRepository {
	return &GormRepository{db: db}
}

func (r *GormRepository) Create(ctx context.Context, u *User) error {
	if err := gorm.G[User](r.db).Create(ctx, u); err != nil {
		if isUniqueViolation(err, "users_email_key") {
			return ErrEmailAlreadyExists
		}
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

// isUniqueViolation 检查错误是否为唯一约束冲突
// 当错误为 pgconn.PgError 且代码为 "23505"（唯一约束冲突）且约束名称与指定的约束名称相同时返回 true
func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}

	return pgErr.Code == "23505" && pgErr.ConstraintName == constraint
}
