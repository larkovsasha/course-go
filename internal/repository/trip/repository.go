package trip

import (
	"context"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	domaintrip "github.com/larkovsasha/course-go/internal/domain/trip"
	"github.com/larkovsasha/course-go/internal/infra/postgres"
)

type Repository struct {
	pool    *pgxpool.Pool
	timeout time.Duration
}

func NewRepository(pool *pgxpool.Pool, timeout time.Duration) *Repository {
	return &Repository{pool: pool, timeout: timeout}
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (domaintrip.Trip, error) {
	executor := postgres.GetExecutor(ctx, r.pool)
	query := sq.Select(
		"id",
		"user_id",
		"driver_id",
		"start_latitude",
		"start_longitude",
		"end_latitude",
		"end_longitude",
		"price",
		"status",
		"started_at",
		"finished_at",
		"created_at",
		"updated_at",
	).
		From("trips").
		Where(sq.Eq{"id": id}).
		PlaceholderFormat(sq.Dollar)

	querySQL, args, err := query.ToSql()
	if err != nil {
		return domaintrip.Trip{}, fmt.Errorf("build get trip query: %w", err)
	}

	queryCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	var result domaintrip.Trip
	err = executor.QueryRow(queryCtx, querySQL, args...).Scan(
		&result.ID,
		&result.UserID,
		&result.DriverID,
		&result.StartLatitude,
		&result.StartLongitude,
		&result.EndLatitude,
		&result.EndLongitude,
		&result.Price,
		&result.Status,
		&result.StartedAt,
		&result.FinishedAt,
		&result.CreatedAt,
		&result.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domaintrip.Trip{}, domaintrip.ErrTripNotFound
	}
	if err != nil {
		return domaintrip.Trip{}, fmt.Errorf("get trip by ID: %w", err)
	}

	return result, nil
}

func (r *Repository) Create(ctx context.Context, value domaintrip.Trip) error {
	executor := postgres.GetExecutor(ctx, r.pool)
	query := sq.Insert("trips").
		Columns(
			"id",
			"user_id",
			"driver_id",
			"start_latitude",
			"start_longitude",
			"end_latitude",
			"end_longitude",
			"price",
			"status",
			"started_at",
			"finished_at",
			"created_at",
			"updated_at",
		).
		Values(
			value.ID,
			value.UserID,
			value.DriverID,
			value.StartLatitude,
			value.StartLongitude,
			value.EndLatitude,
			value.EndLongitude,
			value.Price,
			value.Status,
			value.StartedAt,
			value.FinishedAt,
			value.CreatedAt,
			value.UpdatedAt,
		).
		PlaceholderFormat(sq.Dollar)

	querySQL, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("build create trip query: %w", err)
	}

	queryCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	_, err = executor.Exec(queryCtx, querySQL, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) &&
			pgErr.Code == "23505" &&
			pgErr.ConstraintName == "trips_one_active_per_driver" {
			return domaintrip.ErrDriverBusy
		}

		return fmt.Errorf("create trip: %w", err)
	}

	return nil
}

func (r *Repository) Finish(ctx context.Context, value domaintrip.Trip) error {
	executor := postgres.GetExecutor(ctx, r.pool)
	query := sq.Update("trips").
		Set("updated_at", value.UpdatedAt).
		Set("finished_at", value.FinishedAt).
		Set("status", value.Status).
		Where(sq.Eq{"id": value.ID, "status": domaintrip.Active}).
		PlaceholderFormat(sq.Dollar)

	querySQL, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("build finish trip query: %w", err)
	}

	queryCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	tag, err := executor.Exec(queryCtx, querySQL, args...)
	if err != nil {
		return fmt.Errorf("finish trip: %w", err)
	}
	if tag.RowsAffected() != 0 {
		return nil
	}

	if _, err := r.GetByID(ctx, value.ID); err != nil {
		return err
	}

	return domaintrip.ErrTripCompleted
}

func (r *Repository) CountActive(ctx context.Context) (int64, error) {
	executor := postgres.GetExecutor(ctx, r.pool)
	query := sq.Select("COUNT(*)").
		From("trips").
		Where(sq.Eq{"status": domaintrip.Active}).
		PlaceholderFormat(sq.Dollar)

	querySQL, args, err := query.ToSql()
	if err != nil {
		return 0, fmt.Errorf("build count active trips query: %w", err)
	}

	queryCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	var count int64
	if err := executor.QueryRow(queryCtx, querySQL, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count active trips: %w", err)
	}

	return count, nil
}
