package idempotencycleanup

import "context"

type repository interface {
	DeleteExpired(ctx context.Context) (int64, error)
}
