//go:generate go tool mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test

package create

import (
	"context"

	domaintrip "github.com/larkovsasha/course-go/internal/domain/trip"
	createusecase "github.com/larkovsasha/course-go/internal/usecase/trip/create"
)

type usecase interface {
	Execute(ctx context.Context, input createusecase.Input) (domaintrip.Trip, bool, error)
}
