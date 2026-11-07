//go:build integration

package trip_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	domaintrip "github.com/larkovsasha/course-go/internal/domain/trip"
	"github.com/larkovsasha/course-go/internal/infra/postgres/transaction"
	triprepository "github.com/larkovsasha/course-go/internal/repository/trip"
)

const testQueryTimeout = 3 * time.Second

func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("DATABASE_URL is required for integration tests")
	}

	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("create test database pool: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := pool.Ping(context.Background()); err != nil {
		t.Fatalf("ping test database: %v", err)
	}

	return pool
}

func validTrip() domaintrip.Trip {
	now := time.Now().UTC()
	return domaintrip.Trip{
		ID:             uuid.New(),
		UserID:         uuid.New(),
		DriverID:       uuid.New(),
		StartLatitude:  59.9398,
		StartLongitude: 30.3146,
		EndLatitude:    59.9290,
		EndLongitude:   30.3626,
		Price:          1450,
		Status:         domaintrip.Active,
		StartedAt:      now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func cleanupTrip(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) {
	t.Helper()
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM trips WHERE id = $1", id); err != nil {
			t.Errorf("delete test trip: %v", err)
		}
	})
}

func TestRepositoryCreateAndGet(t *testing.T) {
	pool := newTestPool(t)
	repository := triprepository.NewRepository(pool, testQueryTimeout)
	value := validTrip()
	cleanupTrip(t, pool, value.ID)

	if err := repository.Create(context.Background(), value); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	result, err := repository.GetByID(context.Background(), value.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if result.ID != value.ID {
		t.Errorf("GetByID() ID = %s, want %s", result.ID, value.ID)
	}
	if result.Status != domaintrip.Active {
		t.Errorf("GetByID() status = %s, want %s", result.Status, domaintrip.Active)
	}
}

func TestRepositoryDriverBusy(t *testing.T) {
	pool := newTestPool(t)
	repository := triprepository.NewRepository(pool, testQueryTimeout)
	firstTrip := validTrip()
	secondTrip := validTrip()
	secondTrip.DriverID = firstTrip.DriverID
	cleanupTrip(t, pool, firstTrip.ID)
	cleanupTrip(t, pool, secondTrip.ID)

	if err := repository.Create(context.Background(), firstTrip); err != nil {
		t.Fatalf("create first trip: %v", err)
	}

	err := repository.Create(context.Background(), secondTrip)
	if !errors.Is(err, domaintrip.ErrDriverBusy) {
		t.Fatalf("Create() error = %v, want %v", err, domaintrip.ErrDriverBusy)
	}
}

func TestTransactionRollback(t *testing.T) {
	pool := newTestPool(t)
	repository := triprepository.NewRepository(pool, testQueryTimeout)
	historyRepository := triprepository.NewHistoryRepository(pool, testQueryTimeout)
	manager := transaction.NewManager(pool, testQueryTimeout)
	value := validTrip()
	cleanupTrip(t, pool, value.ID)

	err := manager.Do(context.Background(), func(ctx context.Context) error {
		if err := repository.Create(ctx, value); err != nil {
			return err
		}

		return historyRepository.Add(ctx, domaintrip.StatusChange{
			TripID:    value.ID,
			ToStatus:  domaintrip.Status("invalid"),
			ChangedAt: time.Now().UTC(),
		})
	})
	if err == nil {
		t.Fatal("transaction error = nil, want history constraint error")
	}

	_, err = repository.GetByID(context.Background(), value.ID)
	if !errors.Is(err, domaintrip.ErrTripNotFound) {
		t.Fatalf("GetByID() after rollback error = %v, want %v", err, domaintrip.ErrTripNotFound)
	}
}
