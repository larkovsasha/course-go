package transaction

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/larkovsasha/course-go/internal/infra/postgres"
)

type Manager struct {
	pool    *pgxpool.Pool
	timeout time.Duration
}

func NewManager(pool *pgxpool.Pool, timeout time.Duration) *Manager {
	return &Manager{pool: pool, timeout: timeout}
}

func (m *Manager) Do(ctx context.Context, callback func(context.Context) error) (resultErr error) {
	if postgres.HasTx(ctx) {
		return callback(ctx)
	}

	beginCtx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	tx, err := m.pool.BeginTx(beginCtx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("start transaction: %w", err)
	}

	defer func() {
		panicValue := recover()
		if panicValue == nil && resultErr == nil {
			return
		}

		rollbackCtx, rollbackCancel := context.WithTimeout(context.WithoutCancel(ctx), m.timeout)
		defer rollbackCancel()

		rollbackErr := tx.Rollback(rollbackCtx)
		if panicValue != nil {
			panic(panicValue)
		}

		if rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			resultErr = errors.Join(resultErr, fmt.Errorf("rollback transaction: %w", rollbackErr))
		}
	}()

	if err := callback(postgres.WithTx(ctx, tx)); err != nil {
		return err
	}

	commitCtx, commitCancel := context.WithTimeout(ctx, m.timeout)
	defer commitCancel()

	if err := tx.Commit(commitCtx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
