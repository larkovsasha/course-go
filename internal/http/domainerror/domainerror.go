package domainerror

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/larkovsasha/course-go/internal/domain/trip"
	api "github.com/larkovsasha/course-go/internal/generated"
)

func Problem(err error, instance string) (int, api.Problem) {
	detail := "Internal server error"
	problem := api.Problem{
		Type:     "https://tripgo.example/problems/internal-error",
		Title:    "Internal Server Error",
		Status:   http.StatusInternalServerError,
		Code:     "internal_error",
		Detail:   &detail,
		Instance: &instance,
	}

	var validationError trip.ValidationError
	switch {
	case errors.As(err, &validationError):
		problem.Type = "https://tripgo.example/problems/invalid-request"
		problem.Title = "Invalid request"
		problem.Status = http.StatusBadRequest
		problem.Code = "invalid_request"
		detail = validationError.Error()
	case errors.Is(err, trip.ErrTripNotFound):
		problem.Type = "https://tripgo.example/problems/trip-not-found"
		problem.Title = "Trip not found"
		problem.Status = http.StatusNotFound
		problem.Code = "trip_not_found"
		detail = "Trip was not found"
	case errors.Is(err, trip.ErrTripCompleted):
		problem.Type = "https://tripgo.example/problems/trip-completed"
		problem.Title = "Trip completed"
		problem.Status = http.StatusConflict
		problem.Code = "trip_completed"
		detail = "Operation is not allowed for a completed trip"
	case errors.Is(err, trip.ErrDriverBusy):
		problem.Type = "https://tripgo.example/problems/driver-busy"
		problem.Title = "Driver busy"
		problem.Status = http.StatusConflict
		problem.Code = "driver_busy"
		detail = "Driver already has an active trip"
	case errors.Is(err, trip.ErrIdempotencyConflict):
		problem.Type = "https://tripgo.example/problems/idempotency-conflict"
		problem.Title = "Idempotency conflict"
		problem.Status = http.StatusConflict
		problem.Code = "idempotency_conflict"
		detail = "Idempotency-Key was already used with a different request body"
	}

	return int(problem.Status), problem
}

func Write(
	ctx context.Context,
	w http.ResponseWriter,
	err error,
	instance string,
	logger *slog.Logger,
) {
	status, problem := Problem(err, instance)
	if status >= http.StatusInternalServerError {
		logger.ErrorContext(ctx, "request failed", "path", instance, "error", err)
	}

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	if encodeErr := json.NewEncoder(w).Encode(problem); encodeErr != nil {
		logger.ErrorContext(ctx, "write problem response", "error", encodeErr)
	}
}
