package create

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/larkovsasha/course-go/internal/domain/trip"
)

type Input struct {
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

type Usecase struct {
	tripRepo        tripRepository
	historyRepo     historyRepository
	idempotencyRepo idempotencyRepository
	txManager       txManager
	logger          *slog.Logger
	idempotencyTTL  time.Duration
}

func New(
	tripRepo tripRepository,
	historyRepo historyRepository,
	idempotencyRepo idempotencyRepository,
	txManager txManager,
	logger *slog.Logger,
	idempotencyTTL time.Duration,
) *Usecase {
	return &Usecase{
		tripRepo:        tripRepo,
		historyRepo:     historyRepo,
		idempotencyRepo: idempotencyRepo,
		txManager:       txManager,
		logger:          logger,
		idempotencyTTL:  idempotencyTTL,
	}
}

func (u *Usecase) Execute(ctx context.Context, input Input) (trip.Trip, bool, error) {
	now := time.Now()
	newTrip := trip.Trip{
		ID:             uuid.New(),
		UserID:         input.UserID,
		DriverID:       input.DriverID,
		StartLatitude:  input.StartLatitude,
		StartLongitude: input.StartLongitude,
		EndLatitude:    input.EndLatitude,
		EndLongitude:   input.EndLongitude,
		Price:          input.Price,
		Status:         trip.Active,
		StartedAt:      now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	result := newTrip
	replayed := false
	err := u.txManager.Do(ctx, func(txCtx context.Context) error {
		if input.IdempotencyKey != nil {
			record := trip.IdempotencyRecord{
				Key:         *input.IdempotencyKey,
				RequestHash: input.RequestHash,
				TripID:      newTrip.ID,
				ExpiresAt:   now.Add(u.idempotencyTTL),
			}

			existing, existingRecord, err := u.reserveOrLoadExisting(txCtx, record)
			if err != nil {
				return err
			}
			if existingRecord {
				result = existing
				replayed = true
				return nil
			}
		}

		if err := u.tripRepo.Create(txCtx, newTrip); err != nil {
			return fmt.Errorf("create trip: %w", err)
		}

		change := trip.StatusChange{
			TripID:    newTrip.ID,
			ToStatus:  trip.Active,
			ChangedAt: now,
		}
		if err := u.historyRepo.Add(txCtx, change); err != nil {
			return fmt.Errorf("add initial trip status: %w", err)
		}

		return nil
	})
	if err != nil {
		return trip.Trip{}, false, fmt.Errorf("create trip transaction: %w", err)
	}

	if !replayed {
		u.logger.InfoContext(ctx, "trip created", "trip_id", result.ID.String())
	}

	return result, replayed, nil
}

func (u *Usecase) reserveOrLoadExisting(
	ctx context.Context,
	record trip.IdempotencyRecord,
) (trip.Trip, bool, error) {
	inserted, err := u.idempotencyRepo.Create(ctx, record)
	if err != nil {
		return trip.Trip{}, false, fmt.Errorf("create idempotency record: %w", err)
	}
	if inserted {
		return trip.Trip{}, false, nil
	}

	existingRecord, err := u.idempotencyRepo.GetByKey(ctx, record.Key)
	if err != nil {
		return trip.Trip{}, false, fmt.Errorf("get idempotency record: %w", err)
	}
	if existingRecord.RequestHash != record.RequestHash {
		return trip.Trip{}, false, trip.ErrIdempotencyConflict
	}

	existingTrip, err := u.tripRepo.GetByID(ctx, existingRecord.TripID)
	if err != nil {
		return trip.Trip{}, false, fmt.Errorf("get trip for idempotency record: %w", err)
	}

	return existingTrip, true, nil
}
