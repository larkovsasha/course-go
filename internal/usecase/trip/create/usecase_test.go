package create_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/larkovsasha/course-go/internal/domain/trip"
	"github.com/larkovsasha/course-go/internal/usecase/trip/create"
)

type createDependencies struct {
	tripRepo        *MocktripRepository
	historyRepo     *MockhistoryRepository
	idempotencyRepo *MockidempotencyRepository
	txManager       *MocktxManager
	metrics         *MocktripMetrics
	usecase         *create.Usecase
}

func newCreateDependencies(t *testing.T) createDependencies {
	t.Helper()

	controller := gomock.NewController(t)
	tripRepo := NewMocktripRepository(controller)
	historyRepo := NewMockhistoryRepository(controller)
	idempotencyRepo := NewMockidempotencyRepository(controller)
	txManager := NewMocktxManager(controller)
	metrics := NewMocktripMetrics(controller)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	return createDependencies{
		tripRepo:        tripRepo,
		historyRepo:     historyRepo,
		idempotencyRepo: idempotencyRepo,
		txManager:       txManager,
		metrics:         metrics,
		usecase: create.New(
			tripRepo,
			historyRepo,
			idempotencyRepo,
			txManager,
			metrics,
			logger,
			24*time.Hour,
		),
	}
}

func executeTransaction(mock *MocktxManager) {
	mock.EXPECT().
		Do(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, callback func(context.Context) error) error {
			return callback(ctx)
		})
}

func validCreateInput() create.Input {
	return create.Input{
		UserID:         uuid.New(),
		DriverID:       uuid.New(),
		StartLatitude:  59.9398,
		StartLongitude: 30.3146,
		EndLatitude:    59.9290,
		EndLongitude:   30.3626,
		Price:          1450,
	}
}

func TestUsecaseExecuteSuccess(t *testing.T) {
	dependencies := newCreateDependencies(t)
	executeTransaction(dependencies.txManager)
	dependencies.metrics.EXPECT().TripCreated(gomock.Any())

	var createdTrip trip.Trip
	dependencies.tripRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, value trip.Trip) error {
			createdTrip = value
			return nil
		})
	dependencies.historyRepo.EXPECT().
		Add(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, change trip.StatusChange) error {
			if change.TripID != createdTrip.ID {
				t.Errorf("history trip ID = %s, want %s", change.TripID, createdTrip.ID)
			}
			if change.ToStatus != trip.Active {
				t.Errorf("history status = %s, want %s", change.ToStatus, trip.Active)
			}
			return nil
		})

	input := validCreateInput()
	result, replayed, err := dependencies.usecase.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if replayed {
		t.Errorf("Execute() replayed = true, want false")
	}
	if result.ID == uuid.Nil {
		t.Errorf("Execute() returned empty trip ID")
	}
	if result.Status != trip.Active {
		t.Errorf("Execute() status = %s, want %s", result.Status, trip.Active)
	}
	if result != createdTrip {
		t.Errorf("returned trip differs from the saved trip")
	}
	if result.UserID != input.UserID || result.DriverID != input.DriverID || result.Price != input.Price ||
		result.StartLatitude != input.StartLatitude || result.StartLongitude != input.StartLongitude ||
		result.EndLatitude != input.EndLatitude || result.EndLongitude != input.EndLongitude {
		t.Errorf("created trip lost input fields: %+v", result)
	}
	if result.StartedAt.IsZero() || result.StartedAt != result.CreatedAt || result.StartedAt != result.UpdatedAt || result.FinishedAt != nil {
		t.Errorf("invalid initial trip timestamps: %+v", result)
	}
}

func TestUsecaseExecuteDriverBusy(t *testing.T) {
	dependencies := newCreateDependencies(t)
	executeTransaction(dependencies.txManager)
	dependencies.tripRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(trip.ErrDriverBusy)

	_, _, err := dependencies.usecase.Execute(context.Background(), validCreateInput())
	if !errors.Is(err, trip.ErrDriverBusy) {
		t.Fatalf("Execute() error = %v, want %v", err, trip.ErrDriverBusy)
	}
}

func TestUsecaseExecuteHistoryError(t *testing.T) {
	dependencies := newCreateDependencies(t)
	executeTransaction(dependencies.txManager)
	historyErr := errors.New("history is unavailable")

	dependencies.tripRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	dependencies.historyRepo.EXPECT().Add(gomock.Any(), gomock.Any()).Return(historyErr)

	_, _, err := dependencies.usecase.Execute(context.Background(), validCreateInput())
	if !errors.Is(err, historyErr) {
		t.Fatalf("Execute() error = %v, want wrapped %v", err, historyErr)
	}
}

func TestUsecaseExecuteIdempotencyReplay(t *testing.T) {
	dependencies := newCreateDependencies(t)
	executeTransaction(dependencies.txManager)

	key := uuid.New()
	existingTrip := trip.Trip{ID: uuid.New(), Status: trip.Active}
	input := validCreateInput()
	input.IdempotencyKey = &key
	input.RequestHash = "request-hash"

	dependencies.idempotencyRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(false, nil)
	dependencies.idempotencyRepo.EXPECT().
		GetByKey(gomock.Any(), key).
		Return(trip.IdempotencyRecord{
			Key:         key,
			RequestHash: input.RequestHash,
			TripID:      existingTrip.ID,
		}, nil)
	dependencies.tripRepo.EXPECT().
		GetByID(gomock.Any(), existingTrip.ID).
		Return(existingTrip, nil)

	result, replayed, err := dependencies.usecase.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !replayed {
		t.Errorf("Execute() replayed = false, want true")
	}
	if result.ID != existingTrip.ID {
		t.Errorf("Execute() trip ID = %s, want %s", result.ID, existingTrip.ID)
	}
}

func TestUsecaseExecuteIdempotencyConflict(t *testing.T) {
	dependencies := newCreateDependencies(t)
	executeTransaction(dependencies.txManager)

	key := uuid.New()
	input := validCreateInput()
	input.IdempotencyKey = &key
	input.RequestHash = "new-request-hash"

	dependencies.idempotencyRepo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Return(false, nil)
	dependencies.idempotencyRepo.EXPECT().
		GetByKey(gomock.Any(), key).
		Return(trip.IdempotencyRecord{
			Key:         key,
			RequestHash: "old-request-hash",
		}, nil)

	_, _, err := dependencies.usecase.Execute(context.Background(), input)
	if !errors.Is(err, trip.ErrIdempotencyConflict) {
		t.Fatalf("Execute() error = %v, want %v", err, trip.ErrIdempotencyConflict)
	}
}
