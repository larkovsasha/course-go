package finish_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/larkovsasha/course-go/internal/domain/trip"
	"github.com/larkovsasha/course-go/internal/usecase/trip/finish"
)

type finishDependencies struct {
	tripRepo    *MocktripRepository
	historyRepo *MockhistoryRepository
	txManager   *MocktxManager
	metrics     *MocktripMetrics
	usecase     *finish.Usecase
}

func newFinishDependencies(t *testing.T) finishDependencies {
	t.Helper()

	controller := gomock.NewController(t)
	tripRepo := NewMocktripRepository(controller)
	historyRepo := NewMockhistoryRepository(controller)
	txManager := NewMocktxManager(controller)
	metrics := NewMocktripMetrics(controller)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	return finishDependencies{
		tripRepo:    tripRepo,
		historyRepo: historyRepo,
		txManager:   txManager,
		metrics:     metrics,
		usecase:     finish.New(tripRepo, historyRepo, txManager, metrics, logger),
	}
}

func executeFinishTransaction(mock *MocktxManager) {
	mock.EXPECT().
		Do(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, callback func(context.Context) error) error {
			return callback(ctx)
		})
}

func TestUsecaseExecuteSuccess(t *testing.T) {
	dependencies := newFinishDependencies(t)
	executeFinishTransaction(dependencies.txManager)
	dependencies.metrics.EXPECT().TripCompleted(gomock.Any())
	id := uuid.New()
	activeTrip := trip.Trip{ID: id, Status: trip.Active}

	dependencies.tripRepo.EXPECT().GetByID(gomock.Any(), id).Return(activeTrip, nil)
	dependencies.tripRepo.EXPECT().
		Finish(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, value trip.Trip) error {
			if value.Status != trip.Completed {
				t.Errorf("Finish() status = %s, want %s", value.Status, trip.Completed)
			}
			return nil
		})
	dependencies.historyRepo.EXPECT().
		Add(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, change trip.StatusChange) error {
			if change.ToStatus != trip.Completed {
				t.Errorf("history status = %s, want %s", change.ToStatus, trip.Completed)
			}
			return nil
		})

	result, err := dependencies.usecase.Execute(context.Background(), id)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Status != trip.Completed {
		t.Errorf("Execute() status = %s, want %s", result.Status, trip.Completed)
	}
	if result.FinishedAt == nil {
		t.Errorf("Execute() returned nil finished time")
	}
}

func TestUsecaseExecuteCompleted(t *testing.T) {
	dependencies := newFinishDependencies(t)
	executeFinishTransaction(dependencies.txManager)
	id := uuid.New()
	completedTrip := trip.Trip{ID: id, Status: trip.Completed}

	dependencies.tripRepo.EXPECT().GetByID(gomock.Any(), id).Return(completedTrip, nil)

	_, err := dependencies.usecase.Execute(context.Background(), id)
	if !errors.Is(err, trip.ErrTripCompleted) {
		t.Fatalf("Execute() error = %v, want %v", err, trip.ErrTripCompleted)
	}
}

func TestUsecaseExecuteNotFound(t *testing.T) {
	dependencies := newFinishDependencies(t)
	executeFinishTransaction(dependencies.txManager)
	id := uuid.New()

	dependencies.tripRepo.EXPECT().GetByID(gomock.Any(), id).Return(trip.Trip{}, trip.ErrTripNotFound)

	_, err := dependencies.usecase.Execute(context.Background(), id)
	if !errors.Is(err, trip.ErrTripNotFound) {
		t.Fatalf("Execute() error = %v, want %v", err, trip.ErrTripNotFound)
	}
}

func TestUsecaseExecuteHistoryError(t *testing.T) {
	dependencies := newFinishDependencies(t)
	executeFinishTransaction(dependencies.txManager)
	id := uuid.New()
	historyErr := errors.New("history is unavailable")

	dependencies.tripRepo.EXPECT().GetByID(gomock.Any(), id).Return(trip.Trip{ID: id, Status: trip.Active}, nil)
	dependencies.tripRepo.EXPECT().Finish(gomock.Any(), gomock.Any()).Return(nil)
	dependencies.historyRepo.EXPECT().Add(gomock.Any(), gomock.Any()).Return(historyErr)

	_, err := dependencies.usecase.Execute(context.Background(), id)
	if !errors.Is(err, historyErr) {
		t.Fatalf("Execute() error = %v, want wrapped %v", err, historyErr)
	}
}
