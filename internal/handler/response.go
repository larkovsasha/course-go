package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	api "github.com/larkovsasha/course-go/internal/generated"
	"github.com/larkovsasha/course-go/internal/repository"
	"github.com/larkovsasha/course-go/internal/trip"
)

func writeServiceError(ctx context.Context, w http.ResponseWriter, err error, instance string) {
	detail := "Internal server error"
	problem := api.Problem{
		Type:     "https://tripgo.example/problems/internal-error",
		Title:    "Internal Server Error",
		Status:   http.StatusInternalServerError,
		Code:     "internal_error",
		Detail:   &detail,
		Instance: &instance,
	}

	switch {
	case errors.Is(err, repository.ErrTripNotFound):
		problem.Type = "https://tripgo.example/problems/trip-not-found"
		problem.Title = "Trip not found"
		problem.Status = http.StatusNotFound
		problem.Code = "trip_not_found"
		detail = "Trip was not found"
	case errors.Is(err, repository.ErrTripCompleted):
		problem.Type = "https://tripgo.example/problems/trip-completed"
		problem.Title = "Trip completed"
		problem.Status = http.StatusConflict
		problem.Code = "trip_completed"
		detail = "Operation is not allowed for a completed trip"
	case errors.Is(err, repository.ErrDriverBusy):
		problem.Type = "https://tripgo.example/problems/driver-busy"
		problem.Title = "Driver busy"
		problem.Status = http.StatusConflict
		problem.Code = "driver_busy"
		detail = "Driver already has an active trip"
	case errors.Is(err, repository.ErrIdempotencyConflict):
		problem.Type = "https://tripgo.example/problems/idempotency-conflict"
		problem.Title = "Idempotency conflict"
		problem.Status = http.StatusConflict
		problem.Code = "idempotency_conflict"
		detail = "Idempotency-Key was already used with a different request body"
	default:
		log.Printf("handle request %s: %v", instance, err)
	}

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(int(problem.Status))
	if err := json.NewEncoder(w).Encode(problem); err != nil {
		log.Printf("write problem response: %v", err)
	}
}

func toAPITrip(value trip.Trip) api.Trip {
	return api.Trip{
		Id:       value.ID,
		UserId:   value.UserID,
		DriverId: value.DriverID,
		StartPoint: api.Coordinates{
			Latitude:  value.StartLatitude,
			Longitude: value.StartLongitude,
		},
		EndPoint: api.Coordinates{
			Latitude:  value.EndLatitude,
			Longitude: value.EndLongitude,
		},
		Price:      value.Price,
		Status:     api.TripStatus(value.Status),
		StartedAt:  value.StartedAt,
		FinishedAt: value.FinishedAt,
	}
}

func writeInvalidRequest(ctx context.Context, w http.ResponseWriter, instance string) {
	detail := "Request validation failed"
	problem := api.Problem{
		Type:     "https://tripgo.example/problems/invalid-request",
		Title:    "Invalid request",
		Status:   http.StatusBadRequest,
		Code:     "invalid_request",
		Detail:   &detail,
		Instance: &instance,
	}

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusBadRequest)
	if err := json.NewEncoder(w).Encode(problem); err != nil {
		log.Printf("write invalid request response: %v", err)
	}
}
