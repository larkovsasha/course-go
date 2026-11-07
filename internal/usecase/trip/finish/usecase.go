package finish

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/larkovsasha/course-go/internal/domain/trip"
)

type Usecase struct {
	tripRepo    tripRepository
	historyRepo historyRepository
	txManager   txManager
	metrics     tripMetrics
	logger      *slog.Logger
}

func New(
	tripRepo tripRepository,
	historyRepo historyRepository,
	txManager txManager,
	metrics tripMetrics,
	logger *slog.Logger,
) *Usecase {
	return &Usecase{
		tripRepo:    tripRepo,
		historyRepo: historyRepo,
		txManager:   txManager,
		metrics:     metrics,
		logger:      logger,
	}
}

func (u *Usecase) Execute(ctx context.Context, id uuid.UUID) (trip.Trip, error) {
	var result trip.Trip
	err := u.txManager.Do(ctx, func(txCtx context.Context) error {
		value, err := u.tripRepo.GetByID(txCtx, id)
		if err != nil {
			return fmt.Errorf("get trip for finish: %w", err)
		}

		previousStatus := value.Status
		if err := value.Finish(time.Now()); err != nil {
			return err
		}

		if err := u.tripRepo.Finish(txCtx, value); err != nil {
			return fmt.Errorf("finish trip: %w", err)
		}

		change := trip.StatusChange{
			TripID:     value.ID,
			FromStatus: &previousStatus,
			ToStatus:   value.Status,
			ChangedAt:  *value.FinishedAt,
		}
		if err := u.historyRepo.Add(txCtx, change); err != nil {
			return fmt.Errorf("add completed trip status: %w", err)
		}

		result = value
		return nil
	})
	if err != nil {
		return trip.Trip{}, fmt.Errorf("finish trip transaction: %w", err)
	}

	u.logger.InfoContext(ctx, "trip completed", "trip_id", result.ID.String())
	u.metrics.TripCompleted(ctx)
	return result, nil
}
