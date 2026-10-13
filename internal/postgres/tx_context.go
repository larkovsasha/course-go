package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type txContextKey struct{}

func WithTx(ctx context.Context, tx pgx.Tx) context.Context {
	txContext := context.WithValue(ctx, txContextKey{}, tx)
	return txContext
}

func GetExecutor(ctx context.Context, pool *pgxpool.Pool) DBTx {

	if value := ctx.Value(txContextKey{}); value != nil {
		if tx, ok := value.(pgx.Tx); ok {
			return tx
		}
	}

	return pool
}
