package create

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	api "github.com/larkovsasha/course-go/internal/generated"
	"github.com/larkovsasha/course-go/internal/http/domainerror"
	triphttp "github.com/larkovsasha/course-go/internal/http/trip"
	createusecase "github.com/larkovsasha/course-go/internal/usecase/trip/create"
)

type Handler struct {
	usecase usecase
	logger  *slog.Logger
}

func NewHandler(usecase usecase, logger *slog.Logger) *Handler {
	return &Handler{usecase: usecase, logger: logger}
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		domainerror.Write(r.Context(), w, err, r.URL.Path, h.logger)
		return
	}

	request, err := decode(body)
	if err != nil {
		domainerror.Write(r.Context(), w, err, r.URL.Path, h.logger)
		return
	}
	if err := validate(request); err != nil {
		domainerror.Write(r.Context(), w, err, r.URL.Path, h.logger)
		return
	}

	input := createusecase.Input{
		UserID:         *request.UserID,
		DriverID:       *request.DriverID,
		StartLatitude:  *request.StartPoint.Latitude,
		StartLongitude: *request.StartPoint.Longitude,
		EndLatitude:    *request.EndPoint.Latitude,
		EndLongitude:   *request.EndPoint.Longitude,
		Price:          *request.Price,
	}
	if params.IdempotencyKey != nil {
		hash := sha256.Sum256(body)
		input.IdempotencyKey = params.IdempotencyKey
		input.RequestHash = hex.EncodeToString(hash[:])
	}

	result, replayed, err := h.usecase.Execute(r.Context(), input)
	if err != nil {
		domainerror.Write(r.Context(), w, err, r.URL.Path, h.logger)
		return
	}

	status := http.StatusCreated
	if replayed {
		status = http.StatusOK
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", "/api/v1/trips/"+result.ID.String())
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(triphttp.ToAPI(result)); err != nil {
		h.logger.ErrorContext(r.Context(), "write create trip response", "error", err)
	}
}
