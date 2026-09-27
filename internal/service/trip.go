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
	tripRepo    *repository.TripRepository
	historyRepo *repository.TripStatusHistoryRepository
	txManageer  TxManager
}

type CreateTripInput struct {
	UserID   uuid.UUID
	DriverID uuid.UUID

	StartLatitude  float64
	StartLongitude float64
	EndLatitude    float64
	EndLongitude   float64

	Price int64
}

func NewTripService(
	tripRepo *repository.TripRepository,
	historyRepo *repository.TripStatusHistoryRepository,
	txManageer TxManager,
) *TripService {
	return &TripService{
		tripRepo,
		historyRepo,
		txManageer,
	}
}

func (ts *TripService) Create(ctx context.Context, input CreateTripInput) (trip.Trip, error) {
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

	err := ts.txManageer.Do(ctx, func(txCtx context.Context) error {
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
		return trip.Trip{}, err
	}
	return newTrip, nil
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
