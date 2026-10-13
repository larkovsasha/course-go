package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	api "github.com/larkovsasha/course-go/internal/generated"
	"github.com/larkovsasha/course-go/internal/service"
)

type Handler struct {
	tripService *service.TripService
	pool        *pgxpool.Pool
	dbTimeout   time.Duration
}

func NewHandler(tripService *service.TripService, pool *pgxpool.Pool, dbTimeout time.Duration) *Handler {
	return &Handler{
		tripService,
		pool,
		dbTimeout,
	}
}

func (h *Handler) GetTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	ctx := r.Context()
	trip, err := h.tripService.GetByID(ctx, tripId)

	if err != nil {
		writeServiceError(ctx, w, err, r.URL.Path)
		return
	}

	response := toAPITrip(trip)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("write get trip response: %v", err)
	}
}

func (h *Handler) FinishTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	ctx := r.Context()
	trip, err := h.tripService.Finish(ctx, tripId)
	if err != nil {
		writeServiceError(ctx, w, err, r.URL.Path)
		return
	}

	response := toAPITrip(trip)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("write finish trip response: %v", err)
	}
}

func (h *Handler) CreateTrip(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	ctx := r.Context()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeInvalidRequest(ctx, w, r.URL.Path, "Could not read request body")
		return
	}

	parsedRequest, err := decodeCreateTrip(body)
	if err != nil {
		writeInvalidRequest(ctx, w, r.URL.Path, err.Error())
		return
	}

	err = validateCreateTrip(parsedRequest)
	if err != nil {
		writeInvalidRequest(ctx, w, r.URL.Path, err.Error())
		return
	}

	input := service.CreateTripInput{
		UserID:         *parsedRequest.UserID,
		DriverID:       *parsedRequest.DriverID,
		StartLatitude:  *parsedRequest.StartPoint.Latitude,
		StartLongitude: *parsedRequest.StartPoint.Longitude,
		EndLatitude:    *parsedRequest.EndPoint.Latitude,
		EndLongitude:   *parsedRequest.EndPoint.Longitude,
		Price:          *parsedRequest.Price,
	}
	if params.IdempotencyKey != nil {
		hash := sha256.Sum256(body)
		input.IdempotencyKey = params.IdempotencyKey
		input.RequestHash = string(hex.EncodeToString(hash[:]))
	}

	trip, replayed, err := h.tripService.Create(ctx, input)
	if err != nil {
		writeServiceError(ctx, w, err, r.URL.Path)
		return
	}

	status := http.StatusCreated
	if replayed {
		status = http.StatusOK
	}

	response := toAPITrip(trip)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", "/api/v1/trips/"+trip.ID.String())
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("write create trip response: %v", err)
	}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	response := api.HealthResponse{
		Status: api.Ok,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("write health response: %v", err)
	}
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.dbTimeout)
	defer cancel()

	response := api.HealthResponse{
		Status: api.Ok,
	}
	status := http.StatusOK
	err := h.pool.Ping(ctx)
	if err != nil {
		response.Status = api.Unavailable
		status = http.StatusServiceUnavailable
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("write ready response: %v", err)
	}
}
