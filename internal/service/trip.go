package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/larkovsasha/course-go/internal/repository"
	"github.com/larkovsasha/course-go/internal/trip"
)

type TxManager interface {
	Do(ctx context.Context, callback func(ctx context.Context) error) error
}

type TripService struct {
	tripRepo        *repository.TripRepository
	historyRepo     *repository.TripStatusHistoryRepository
	idempotencyRepo *repository.IdempotencyRepository
	txManageer      TxManager
	idempotencyTTl  time.Duration
}

type CreateTripInput struct {
	UserID   uuid.UUID
	DriverID uuid.UUID

	StartLatitude  float64
	StartLongitude float64
	EndLatitude    float64
	EndLongitude   float64

	Price int64

	IdempotencyKey *uuid.UUID
	RequestHash    string
}

func NewTripService(
	tripRepo *repository.TripRepository,
	historyRepo *repository.TripStatusHistoryRepository,
	idempotencyRepo *repository.IdempotencyRepository,
	txManageer TxManager,
	idempotencyTTl time.Duration,
) *TripService {
	return &TripService{
		tripRepo,
		historyRepo,
		idempotencyRepo,
		txManageer,
		idempotencyTTl,
	}
}

func (ts *TripService) Create(ctx context.Context, input CreateTripInput) (trip.Trip, bool, error) {
	now := time.Now()

	newTrip := trip.Trip{
		ID:       uuid.New(),
		UserID:   input.UserID,
		DriverID: input.DriverID,

		StartLatitude:  input.StartLatitude,
		StartLongitude: input.StartLongitude,
		EndLatitude:    input.EndLatitude,
		EndLongitude:   input.EndLongitude,

		Price:      input.Price,
		Status:     trip.Active,
		StartedAt:  now,
		CreatedAt:  now,
		UpdatedAt:  now,
		FinishedAt: nil,
	}

	tripHistory := trip.StatusChange{
		TripID:     newTrip.ID,
		FromStatus: nil,
		ToStatus:   trip.Active,
		Reason:     nil,
		ChangedAt:  now,
	}

	resultTrip := newTrip
	resultReplayed := false
	err := ts.txManageer.Do(ctx, func(txCtx context.Context) error {
		if input.IdempotencyKey != nil {
			idempotencyRecord := trip.IdempotencyRecord{
				Key:         *input.IdempotencyKey,
				RequestHash: input.RequestHash,
				TripID:      newTrip.ID,
				ExpiresAt:   now.Add(ts.idempotencyTTl),
			}

			existingTrip, replayed, err := ts.reserveOrLoadExisting(txCtx, idempotencyRecord)
			if err != nil {
				return err
			}
			if replayed {
				resultReplayed = true
				resultTrip = existingTrip
				return nil
			}
		}

		err := ts.tripRepo.Create(txCtx, newTrip)
		if err != nil {
			return err
		}
		err = ts.historyRepo.Add(txCtx, tripHistory)
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return trip.Trip{}, false, err
	}
	return resultTrip, resultReplayed, nil
}

func (ts *TripService) GetByID(ctx context.Context, id uuid.UUID) (trip.Trip, error) {
	return ts.tripRepo.GetByID(ctx, id)
}

func (ts *TripService) Finish(ctx context.Context, id uuid.UUID) (trip.Trip, error) {
	now := time.Now()

	finishedTrip := trip.Trip{}
	err := ts.txManageer.Do(ctx, func(txCtx context.Context) error {
		err := ts.tripRepo.Finish(txCtx, id, now)
		if err != nil {
			return err
		}

		activeStatus := trip.Active
		statusChange := trip.StatusChange{
			TripID:     id,
			FromStatus: &activeStatus,
			ToStatus:   trip.Completed,
			ChangedAt:  now,
			Reason:     nil,
		}

		err = ts.historyRepo.Add(txCtx, statusChange)
		if err != nil {
			return err
		}

		finishedTrip, err = ts.GetByID(txCtx, id)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return trip.Trip{}, err
	}

	return finishedTrip, nil
}

func (ts *TripService) reserveOrLoadExisting(
	ctx context.Context,
	idempotencyRecord trip.IdempotencyRecord,
) (trip.Trip, bool, error) {
	keyInserted, err := ts.idempotencyRepo.Create(ctx, idempotencyRecord)
	if err != nil {
		return trip.Trip{}, false, err
	}

	if keyInserted {
		return trip.Trip{}, false, nil
	}

	oldRecord, err := ts.idempotencyRepo.GetByKey(ctx, idempotencyRecord.Key)
	if err != nil {
		return trip.Trip{}, false, err
	}
	if oldRecord.RequestHash != idempotencyRecord.RequestHash {
		return trip.Trip{}, false, repository.ErrIdempotencyConflict
	}

	existingTrip, err := ts.tripRepo.GetByID(ctx, oldRecord.TripID)
	if err != nil {
		return trip.Trip{}, false, err
	}
	return existingTrip, true, nil
}
