package get_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/larkovsasha/course-go/internal/domain/trip"
	"github.com/larkovsasha/course-go/internal/usecase/trip/get"
)

func TestUsecaseExecute(t *testing.T) {
	repositoryErr := errors.New("repository unavailable")
	tests := []struct {
		name    string
		result  trip.Trip
		repoErr error
		wantErr error
	}{
		{
			name: "trip found",
			result: trip.Trip{
				ID: uuid.New(), UserID: uuid.New(), DriverID: uuid.New(), Status: trip.Active,
				StartLatitude: 59.9, StartLongitude: 30.3, EndLatitude: 60, EndLongitude: 30.4,
				Price: 1450, StartedAt: time.Now(), CreatedAt: time.Now(), UpdatedAt: time.Now(),
			},
		},
		{
			name:    "trip not found",
			repoErr: trip.ErrTripNotFound,
			wantErr: trip.ErrTripNotFound,
		},
		{
			name:    "repository failure discards partial result",
			result:  trip.Trip{ID: uuid.New(), Status: trip.Active},
			repoErr: repositoryErr,
			wantErr: repositoryErr,
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
			wantResult := test.result
			if test.wantErr != nil {
				wantResult = trip.Trip{}
			}
			if result != wantResult {
				t.Errorf("Execute() trip = %+v, want %+v", result, wantResult)
			}
		})
	}
}
