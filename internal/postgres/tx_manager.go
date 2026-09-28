package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TransactionManager struct {
	pool    *pgxpool.Pool
	timeout time.Duration
}

func NewTransactionManager(pool *pgxpool.Pool, timeout time.Duration) *TransactionManager {
	return &TransactionManager{
		pool,
		timeout,
	}
}

func (tm *TransactionManager) Do(ctx context.Context, callback func(context.Context) error) (resultErr error) {
	if _, ok := ctx.Value(txContextKey{}).(pgx.Tx); ok {
		return callback(ctx)
	}

	beginCtx, cbeginCancel := context.WithTimeout(ctx, tm.timeout)
	defer cbeginCancel()

	tx, err := tm.pool.BeginTx(beginCtx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("Failed to start transaction %w", err)
	}

	defer func() {
		rollbackCtx, rollbackCancel := context.WithTimeout(
			context.WithoutCancel(ctx),
			tm.timeout,
		)
		defer rollbackCancel()

		rollbackErr := tx.Rollback(rollbackCtx)
		if rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			resultErr = errors.Join(resultErr, fmt.Errorf("Rollback transaction %w", rollbackErr))
		}
	}()

	err = callback(WithTx(ctx, tx))
	if err != nil {
		return err
	}

	commitCtx, commitCancel := context.WithTimeout(ctx, tm.timeout)
	defer commitCancel()

	commitErr := tx.Commit(commitCtx)
	if commitErr != nil {
		return fmt.Errorf("Failed to commit %w", commitErr)
	}

	return nil
}
