//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test

package create

import (
	"context"

	"github.com/google/uuid"

	"github.com/larkovsasha/course-go/internal/domain/trip"
)

type (
	tripRepository interface {
		Create(ctx context.Context, value trip.Trip) error
		GetByID(ctx context.Context, id uuid.UUID) (trip.Trip, error)
	}

	historyRepository interface {
		Add(ctx context.Context, change trip.StatusChange) error
	}

	idempotencyRepository interface {
		Create(ctx context.Context, record trip.IdempotencyRecord) (bool, error)
		GetByKey(ctx context.Context, key uuid.UUID) (trip.IdempotencyRecord, error)
	}

	txManager interface {
		Do(ctx context.Context, callback func(context.Context) error) error
	}
)
