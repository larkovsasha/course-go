//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test

package get

import (
	"context"

	"github.com/google/uuid"

	domaintrip "github.com/larkovsasha/course-go/internal/domain/trip"
)

type usecase interface {
	Execute(ctx context.Context, id uuid.UUID) (domaintrip.Trip, error)
}
