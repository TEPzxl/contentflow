package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

var ErrRefreshTokenNotFound = errors.New("refresh tokrn not found")

type RefreshTokenRepository interface {
	Create(ctx context.Context, token *RefreshToken) error
	FindValidByHash(ctx context.Context, tokenHash string, now time.Time) (*RefreshToken, error)
	RevokeByHash(ctx context.Context, tokenHash string, revokedAt time.Time) error
	RevokeAllByUserID(ctx context.Context, userID int64, revokedAt time.Time) error
}

type GormRefreshTokenRepository struct {
	db *gorm.DB
}

func NewGormRefreshTokenRepository(db *gorm.DB) *GormRefreshTokenRepository {
	return &GormRefreshTokenRepository{db: db}
}

func (r *GormRefreshTokenRepository) Create(ctx context.Context, token *RefreshToken) error {
	if err := gorm.G[RefreshToken](r.db).Create(ctx, token); err != nil {
		return fmt.Errorf("create refresh token: %w", err)
	}
	return nil
}

func (r *GormRefreshTokenRepository) FindValidByHash(ctx context.Context, tokenHash string, now time.Time) (*RefreshToken, error) {
	token, err := gorm.G[RefreshToken](r.db).
		Where("token_hash = ?", tokenHash).
		Where("revoked_at IS NULL").
		Where("expires_at > ?", now).
		First(ctx)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrRefreshTokenNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("find valid refresh token by hash: %w", err)
	}
	return &token, nil
}

func (r *GormRefreshTokenRepository) RevokeByHash(ctx context.Context, tokenHash string, revokedAt time.Time) error {
	rowsAffected, err := gorm.G[RefreshToken](r.db).
		Where("token_hash = ?", tokenHash).
		Update(ctx, "revoked_at", revokedAt)
	if err != nil {
		return fmt.Errorf("revoke refresh token by hash: %w", err)
	}

	if rowsAffected == 0 {
		return ErrRefreshTokenNotFound
	}
	return nil
}

func (r *GormRefreshTokenRepository) RevokeAllByUserID(ctx context.Context, userID int64, revokedAt time.Time) error {
	_, err := gorm.G[RefreshToken](r.db).
		Where("user_id = ?", userID).
		Update(ctx, "revoked_at", revokedAt)
	if err != nil {
		return fmt.Errorf("revoke all refresh tokens by user id: %w", err)
	}
	return nil
}
