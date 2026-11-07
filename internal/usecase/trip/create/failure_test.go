package create_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/larkovsasha/course-go/internal/domain/trip"
)

func TestUsecaseExecuteNewIdempotencyKey(t *testing.T) {
	dependencies := newCreateDependencies(t)
	executeTransaction(dependencies.txManager)
	input := validCreateInput()
	key := uuid.New()
	input.IdempotencyKey = &key
	input.RequestHash = "first-request"
	var record trip.IdempotencyRecord
	var saved trip.Trip
	gomock.InOrder(
		dependencies.idempotencyRepo.EXPECT().Create(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, value trip.IdempotencyRecord) (bool, error) {
				record = value
				return true, nil
			}),
		dependencies.tripRepo.EXPECT().Create(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, value trip.Trip) error {
				saved = value
				return nil
			}),
		dependencies.historyRepo.EXPECT().Add(gomock.Any(), gomock.Any()).Return(nil),
		dependencies.metrics.EXPECT().TripCreated(gomock.Any()),
	)
	before := time.Now()
	result, replayed, err := dependencies.usecase.Execute(context.Background(), input)
	after := time.Now()
	if err != nil || replayed || result.ID == uuid.Nil || result != saved {
		t.Fatalf("Execute() = (%+v, %t, %v), saved = %+v", result, replayed, err, saved)
	}
	if record.Key != key || record.RequestHash != input.RequestHash || record.TripID != result.ID {
		t.Errorf("idempotency record does not describe the created trip: %+v", record)
	}
	if record.ExpiresAt.Before(before.Add(24*time.Hour)) || record.ExpiresAt.After(after.Add(24*time.Hour)) {
		t.Errorf("idempotency expiry = %v, want creation time + 24h", record.ExpiresAt)
	}
}

func TestUsecaseExecuteIdempotencyDependencyFailures(t *testing.T) {
	failure := errors.New("storage unavailable")
	for _, step := range []string{"reserve key", "load key", "load trip"} {
		t.Run(step, func(t *testing.T) {
			dependencies := newCreateDependencies(t)
			executeTransaction(dependencies.txManager)
			input := validCreateInput()
			key := uuid.New()
			input.IdempotencyKey = &key
			input.RequestHash = "same-request"
			if step == "reserve key" {
				dependencies.idempotencyRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(false, failure)
			} else {
				dependencies.idempotencyRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(false, nil)
				if step == "load key" {
					dependencies.idempotencyRepo.EXPECT().GetByKey(gomock.Any(), key).Return(trip.IdempotencyRecord{}, failure)
				} else {
					tripID := uuid.New()
					dependencies.idempotencyRepo.EXPECT().GetByKey(gomock.Any(), key).
						Return(trip.IdempotencyRecord{RequestHash: input.RequestHash, TripID: tripID}, nil)
					dependencies.tripRepo.EXPECT().GetByID(gomock.Any(), tripID).Return(trip.Trip{}, failure)
				}
			}
			result, replayed, err := dependencies.usecase.Execute(context.Background(), input)
			if !errors.Is(err, failure) || result != (trip.Trip{}) || replayed {
				t.Errorf("Execute() = (%+v, %t, %v), want empty trip, false and storage error", result, replayed, err)
			}
		})
	}
}

func TestUsecaseExecuteCommitFailure(t *testing.T) {
	dependencies := newCreateDependencies(t)
	commitErr := errors.New("commit failed")
	dependencies.txManager.EXPECT().Do(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, callback func(context.Context) error) error {
			if err := callback(ctx); err != nil {
				return err
			}
			return commitErr
		})
	dependencies.tripRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	dependencies.historyRepo.EXPECT().Add(gomock.Any(), gomock.Any()).Return(nil)
	result, replayed, err := dependencies.usecase.Execute(context.Background(), validCreateInput())
	if !errors.Is(err, commitErr) || result != (trip.Trip{}) || replayed {
		t.Errorf("Execute() = (%+v, %t, %v), want empty trip, false and commit error", result, replayed, err)
	}
}
