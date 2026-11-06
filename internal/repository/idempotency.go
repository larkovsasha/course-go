package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/larkovsasha/course-go/internal/postgres"
	"github.com/larkovsasha/course-go/internal/trip"
)

var ErrIdempotencyConflict = errors.New("Idempotency key already exists with another data")
var ErrKeyNotFound = errors.New("Idempotency key does not exists")

type IdempotencyRepository struct {
	pool    *pgxpool.Pool
	timeout time.Duration
}

func NewIdempotencyRepository(pool *pgxpool.Pool, timeout time.Duration) *IdempotencyRepository {
	return &IdempotencyRepository{
		pool,
		timeout,
	}
}

func (r *IdempotencyRepository) Create(ctx context.Context, record trip.IdempotencyRecord) (bool, error) {
	dbTx := postgres.GetExecutor(ctx, r.pool)

	query := sq.Insert("idempotency_keys").
		Columns(
			"key",
			"request_hash",
			"trip_id",
			"expires_at",
		).
		Values(
			record.Key,
			record.RequestHash,
			record.TripID,
			record.ExpiresAt,
		).
		Suffix(`ON CONFLICT (key) DO UPDATE
			SET request_hash = EXCLUDED.request_hash,
				trip_id = EXCLUDED.trip_id,
				expires_at = EXCLUDED.expires_at
			WHERE idempotency_keys.expires_at <= CURRENT_TIMESTAMP
		`).
		PlaceholderFormat(sq.Dollar)

	querySQL, args, err := query.ToSql()
	if err != nil {
		return false, fmt.Errorf("build create idempotency record query: %w", err)
	}

	timeoutContext, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	tag, err := dbTx.Exec(timeoutContext, querySQL, args...)
	if err != nil {
		return false, fmt.Errorf("create idempotency record: %w", err)
	}

	if tag.RowsAffected() == 1 {
		return true, nil
	}
	return false, nil
}

func (r *IdempotencyRepository) GetByKey(ctx context.Context, key uuid.UUID) (trip.IdempotencyRecord, error) {
	dbTx := postgres.GetExecutor(ctx, r.pool)

	query := sq.Select("key", "request_hash", "trip_id", "expires_at").
		From("idempotency_keys").
		Where(sq.Eq{"key": key}).
		PlaceholderFormat(sq.Dollar)
	querySQL, args, err := query.ToSql()

	if err != nil {
		return trip.IdempotencyRecord{}, fmt.Errorf("build get idempotency record query: %w", err)
	}

	timeoutContext, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	var record trip.IdempotencyRecord
	err = dbTx.QueryRow(timeoutContext, querySQL, args...).Scan(
		&record.Key,
		&record.RequestHash,
		&record.TripID,
		&record.ExpiresAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return trip.IdempotencyRecord{}, ErrKeyNotFound
		}
		return trip.IdempotencyRecord{}, fmt.Errorf("get idempotency record: %w", err)
	}
	return record, nil
}

func (r *IdempotencyRepository) DeleteExpired(ctx context.Context) (int64, error) {
	dbTx := postgres.GetExecutor(ctx, r.pool)

	query := sq.Delete("idempotency_keys").
		Where("expires_at <= NOW()").
		PlaceholderFormat(sq.Dollar)

	querySQL, args, err := query.ToSql()
	if err != nil {
		return 0, fmt.Errorf("build delete expired idempotency records query: %w", err)
	}

	timeoutContext, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	tag, err := dbTx.Exec(timeoutContext, querySQL, args...)
	if err != nil {
		return 0, fmt.Errorf("delete expired idempotency records: %w", err)
	}

	return tag.RowsAffected(), nil
}
