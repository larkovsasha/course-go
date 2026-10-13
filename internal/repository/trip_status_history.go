package repository

import (
	"context"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/larkovsasha/course-go/internal/postgres"
	"github.com/larkovsasha/course-go/internal/trip"
)

type TripStatusHistoryRepository struct {
	pool    *pgxpool.Pool
	timeout time.Duration
}

func NewTripStatusHistoryRepository(pool *pgxpool.Pool, timeout time.Duration) *TripStatusHistoryRepository {
	return &TripStatusHistoryRepository{
		pool,
		timeout,
	}
}
func (r *TripStatusHistoryRepository) Add(ctx context.Context, status trip.StatusChange) error {
	dbTx := postgres.GetExecutor(ctx, r.pool)

	query := sq.Insert("trip_status_history").
		Columns(
			"trip_id",
			"from_status",
			"to_status",
			"reason",
			"changed_at",
		).
		Values(
			status.TripID,
			status.FromStatus,
			status.ToStatus,
			status.Reason,
			status.ChangedAt,
		).
		PlaceholderFormat(sq.Dollar)

	querySQL, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("build add trip hitory query: %w", err)
	}

	timeoutContext, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	_, err = dbTx.Exec(timeoutContext, querySQL, args...)
	if err != nil {
		return fmt.Errorf("create trip hostory: %w", err)
	}

	return nil

}
