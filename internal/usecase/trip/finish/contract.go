//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test

package finish

import (
	"context"

	"github.com/google/uuid"

	"github.com/larkovsasha/course-go/internal/domain/trip"
)

type (
	tripRepository interface {
		GetByID(ctx context.Context, id uuid.UUID) (trip.Trip, error)
		Finish(ctx context.Context, value trip.Trip) error
	}

	historyRepository interface {
		Add(ctx context.Context, change trip.StatusChange) error
	}

	txManager interface {
		Do(ctx context.Context, callback func(context.Context) error) error
	}
)
