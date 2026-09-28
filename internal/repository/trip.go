package repository

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
	"github.com/larkovsasha/course-go/internal/postgres"
	"github.com/larkovsasha/course-go/internal/trip"
)

var ErrTripNotFound = errors.New("trip not found")
var ErrDriverBusy = errors.New("driver is busy")
var ErrTripCompleted = errors.New("trip is completed")

type TripRepository struct {
	pool    *pgxpool.Pool
	timeout time.Duration
}

func NewTripRepository(pool *pgxpool.Pool, timeout time.Duration) *TripRepository {
	return &TripRepository{
		pool,
		timeout,
	}
}

func (r *TripRepository) GetByID(ctx context.Context, id uuid.UUID) (trip.Trip, error) {
	dbTx := postgres.GetExecutor(ctx, r.pool)

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
		return trip.Trip{}, fmt.Errorf("build get trip query: %w", err)
	}

	timeoutContext, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	var result trip.Trip
	err = dbTx.QueryRow(timeoutContext, querySQL, args...).Scan(
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
		return trip.Trip{}, ErrTripNotFound
	}
	if err != nil {
		return trip.Trip{}, fmt.Errorf("get trip by ID: %w", err)
	}

	return result, nil
}

func (r *TripRepository) Create(ctx context.Context, trip trip.Trip) error {
	dbTx := postgres.GetExecutor(ctx, r.pool)

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
			trip.ID,
			trip.UserID,
			trip.DriverID,
			trip.StartLatitude,
			trip.StartLongitude,
			trip.EndLatitude,
			trip.EndLongitude,
			trip.Price,
			trip.Status,
			trip.StartedAt,
			trip.FinishedAt,
			trip.CreatedAt,
			trip.UpdatedAt,
		).
		PlaceholderFormat(sq.Dollar)

	querySQL, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("build create trip query: %w", err)
	}

	timeoutContext, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	_, err = dbTx.Exec(timeoutContext, querySQL, args...)
	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "trips_one_active_per_driver" {
			return ErrDriverBusy
		}
		return fmt.Errorf("create trip: %w", err)
	}

	return nil
}

func (r *TripRepository) Finish(ctx context.Context, id uuid.UUID, finishTime time.Time) error {
	dbTx := postgres.GetExecutor(ctx, r.pool)

	query := sq.Update("trips").
		Set("updated_at", finishTime).
		Set("finished_at", finishTime).
		Set("status", "completed").
		Where(sq.Eq{"id": id, "status": "active"}).
		PlaceholderFormat(sq.Dollar)

	querySQL, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("build finish trip query: %w", err)
	}

	timeoutContext, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	tag, err := dbTx.Exec(timeoutContext, querySQL, args...)
	if err != nil {
		return fmt.Errorf("finish trip: %w", err)
	}

	if tag.RowsAffected() == 0 {
		_, err := r.GetByID(ctx, id)
		if err != nil {
			return err
		}
		return ErrTripCompleted
	}

	return nil
}
