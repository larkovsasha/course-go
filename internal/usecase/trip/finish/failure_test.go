package finish_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/larkovsasha/course-go/internal/domain/trip"
)

func TestUsecaseExecuteConcurrentFinish(t *testing.T) {
	dependencies := newFinishDependencies(t)
	executeFinishTransaction(dependencies.txManager)
	id := uuid.New()
	dependencies.tripRepo.EXPECT().GetByID(gomock.Any(), id).Return(trip.Trip{ID: id, Status: trip.Active}, nil)
	dependencies.tripRepo.EXPECT().Finish(gomock.Any(), gomock.Any()).Return(trip.ErrTripCompleted)
	result, err := dependencies.usecase.Execute(context.Background(), id)
	if !errors.Is(err, trip.ErrTripCompleted) || result != (trip.Trip{}) {
		t.Errorf("Execute() = (%+v, %v), want empty trip and completed error", result, err)
	}
}

func TestUsecaseExecuteCommitFailure(t *testing.T) {
	dependencies := newFinishDependencies(t)
	commitErr := errors.New("commit failed")
	id := uuid.New()
	dependencies.txManager.EXPECT().Do(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, callback func(context.Context) error) error {
			if err := callback(ctx); err != nil {
				return err
			}
			return commitErr
		})
	dependencies.tripRepo.EXPECT().GetByID(gomock.Any(), id).Return(trip.Trip{ID: id, Status: trip.Active}, nil)
	dependencies.tripRepo.EXPECT().Finish(gomock.Any(), gomock.Any()).Return(nil)
	dependencies.historyRepo.EXPECT().Add(gomock.Any(), gomock.Any()).Return(nil)
	result, err := dependencies.usecase.Execute(context.Background(), id)
	if !errors.Is(err, commitErr) || result != (trip.Trip{}) {
		t.Errorf("Execute() = (%+v, %v), want empty trip and commit error", result, err)
	}
}
