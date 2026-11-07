package trip_test

import (
	"errors"
	"testing"
	"time"

	"github.com/larkovsasha/course-go/internal/domain/trip"
)

func TestTripFinish(t *testing.T) {
	finishedAt := time.Date(2026, time.November, 7, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		status     trip.Status
		wantStatus trip.Status
		wantErr    error
	}{
		{
			name:       "active trip",
			status:     trip.Active,
			wantStatus: trip.Completed,
		},
		{
			name:       "completed trip",
			status:     trip.Completed,
			wantStatus: trip.Completed,
			wantErr:    trip.ErrTripCompleted,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := trip.Trip{Status: test.status}

			err := value.Finish(finishedAt)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Finish() error = %v, want %v", err, test.wantErr)
			}
			if value.Status != test.wantStatus {
				t.Errorf("Finish() status = %s, want %s", value.Status, test.wantStatus)
			}

			if test.wantErr != nil {
				if value.FinishedAt != nil {
					t.Errorf("Finish() changed finished time for completed trip")
				}
				return
			}

			if value.FinishedAt == nil || !value.FinishedAt.Equal(finishedAt) {
				t.Errorf("Finish() finished time = %v, want %v", value.FinishedAt, finishedAt)
			}
			if !value.UpdatedAt.Equal(finishedAt) {
				t.Errorf("Finish() updated time = %v, want %v", value.UpdatedAt, finishedAt)
			}
		})
	}
}

func TestValidationError(t *testing.T) {
	tests := []struct {
		name  string
		err   trip.ValidationError
		value string
	}{
		{
			name:  "field error",
			err:   trip.ValidationError{Field: "price", Reason: "must be positive"},
			value: "price: must be positive",
		},
		{
			name:  "common error",
			err:   trip.ValidationError{Reason: "invalid request"},
			value: "invalid request",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.err.Error() != test.value {
				t.Errorf("Error() = %q, want %q", test.err.Error(), test.value)
			}
		})
	}
}
