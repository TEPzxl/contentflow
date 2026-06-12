package dbtx

import (
	"context"

	"gorm.io/gorm"
)

type contextKey struct{}

func WithTx(ctx context.Context, tx *gorm.DB) context.Context {
	if tx == nil {
		return ctx
	}
	return context.WithValue(ctx, contextKey{}, tx)
}

func FromContext(ctx context.Context, fallback *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(contextKey{}).(*gorm.DB); ok && tx != nil {
		return tx
	}
	return fallback
}
