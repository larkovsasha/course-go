package trip

import (
	"context"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgxpool"

	domaintrip "github.com/larkovsasha/course-go/internal/domain/trip"
	"github.com/larkovsasha/course-go/internal/infra/postgres"
)

type HistoryRepository struct {
	pool    *pgxpool.Pool
	timeout time.Duration
}

func NewHistoryRepository(pool *pgxpool.Pool, timeout time.Duration) *HistoryRepository {
	return &HistoryRepository{pool: pool, timeout: timeout}
}

func (r *HistoryRepository) Add(ctx context.Context, change domaintrip.StatusChange) error {
	executor := postgres.GetExecutor(ctx, r.pool)
	query := sq.Insert("trip_status_history").
		Columns("trip_id", "from_status", "to_status", "reason", "changed_at").
		Values(
			change.TripID,
			change.FromStatus,
			change.ToStatus,
			change.Reason,
			change.ChangedAt,
		).
		PlaceholderFormat(sq.Dollar)

	querySQL, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("build add trip history query: %w", err)
	}

	queryCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	if _, err := executor.Exec(queryCtx, querySQL, args...); err != nil {
		return fmt.Errorf("add trip history: %w", err)
	}

	return nil
}
