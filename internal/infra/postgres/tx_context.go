package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type txContextKey struct{}

func WithTx(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, txContextKey{}, tx)
}

func HasTx(ctx context.Context) bool {
	_, ok := ctx.Value(txContextKey{}).(pgx.Tx)
	return ok
}

func GetExecutor(ctx context.Context, pool *pgxpool.Pool) Executor {
	if tx, ok := ctx.Value(txContextKey{}).(pgx.Tx); ok {
		return tx
	}

	return pool
}
