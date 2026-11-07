package domainerror_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/larkovsasha/course-go/internal/domain/trip"
	"github.com/larkovsasha/course-go/internal/http/domainerror"
)

func TestProblem(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "validation error",
			err:        trip.ValidationError{Field: "price", Reason: "invalid"},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:       "trip not found",
			err:        trip.ErrTripNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   "trip_not_found",
		},
		{
			name:       "trip completed",
			err:        trip.ErrTripCompleted,
			wantStatus: http.StatusConflict,
			wantCode:   "trip_completed",
		},
		{
			name:       "driver busy",
			err:        trip.ErrDriverBusy,
			wantStatus: http.StatusConflict,
			wantCode:   "driver_busy",
		},
		{
			name:       "idempotency conflict",
			err:        trip.ErrIdempotencyConflict,
			wantStatus: http.StatusConflict,
			wantCode:   "idempotency_conflict",
		},
		{
			name:       "internal error",
			err:        errors.New("database password leaked"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal_error",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, problem := domainerror.Problem(test.err, "/api/v1/trips")
			if status != test.wantStatus {
				t.Errorf("Problem() status = %d, want %d", status, test.wantStatus)
			}
			if problem.Code != test.wantCode {
				t.Errorf("Problem() code = %s, want %s", problem.Code, test.wantCode)
			}
			if test.wantStatus == http.StatusInternalServerError &&
				problem.Detail != nil && *problem.Detail == test.err.Error() {
				t.Errorf("Problem() exposed internal error")
			}
		})
	}
}
