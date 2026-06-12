package collector

import (
	"context"
	"fmt"

	"github.com/tepzxl/contentflow/internal/storage/dbtx"
	"gorm.io/gorm"
)

type TransactionRunner interface {
	RunInTransaction(ctx context.Context, fn func(context.Context) error) error
}

type GormTransactionRunner struct {
	db *gorm.DB
}

func NewGormTransactionRunner(db *gorm.DB) *GormTransactionRunner {
	return &GormTransactionRunner{db: db}
}

func (r *GormTransactionRunner) RunInTransaction(ctx context.Context, fn func(context.Context) error) error {
	if r == nil || r.db == nil {
		return fn(ctx)
	}

	if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(dbtx.WithTx(ctx, tx))
	}); err != nil {
		return fmt.Errorf("run transaction: %w", err)
	}
	return nil
}
