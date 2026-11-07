//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test

package get

import (
	"context"

	"github.com/google/uuid"

	"github.com/larkovsasha/course-go/internal/domain/trip"
)

type tripRepository interface {
	Create(ctx context.Context, value trip.Trip) error
	GetByID(ctx context.Context, id uuid.UUID) (trip.Trip, error)
	Finish(ctx context.Context, value trip.Trip) error
}
