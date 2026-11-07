package get_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/larkovsasha/course-go/internal/domain/trip"
	"github.com/larkovsasha/course-go/internal/usecase/trip/get"
)

func TestUsecaseExecute(t *testing.T) {
	tests := []struct {
		name    string
		result  trip.Trip
		repoErr error
		wantErr error
	}{
		{
			name:   "trip found",
			result: trip.Trip{ID: uuid.New(), Status: trip.Active},
		},
		{
			name:    "trip not found",
			repoErr: trip.ErrTripNotFound,
			wantErr: trip.ErrTripNotFound,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			controller := gomock.NewController(t)
			repository := NewMocktripRepository(controller)
			usecase := get.New(repository)
			id := uuid.New()

			repository.EXPECT().GetByID(gomock.Any(), id).Return(test.result, test.repoErr)

			result, err := usecase.Execute(context.Background(), id)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Execute() error = %v, want %v", err, test.wantErr)
			}
			if err == nil && result.ID != test.result.ID {
				t.Errorf("Execute() trip ID = %s, want %s", result.ID, test.result.ID)
			}
		})
	}
}
