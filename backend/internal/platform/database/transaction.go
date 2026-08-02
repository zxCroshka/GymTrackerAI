package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Beginner is the narrow transaction capability implemented by pgxpool.Pool.
// It also allows transaction orchestration to be tested without a generic
// repository abstraction.
type Beginner interface {
	BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error)
}

// WithinTransaction runs fn in one transaction. A callback error or panic
// rolls the transaction back; commit errors are returned to the caller.
func WithinTransaction(ctx context.Context, beginner Beginner, options pgx.TxOptions, fn func(pgx.Tx) error) error {
	tx, err := beginner.BeginTx(ctx, options)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}
