package finish

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/larkovsasha/course-go/internal/http/domainerror"
	triphttp "github.com/larkovsasha/course-go/internal/http/trip"
)

type Handler struct {
	usecase usecase
	logger  *slog.Logger
}

func NewHandler(usecase usecase, logger *slog.Logger) *Handler {
	return &Handler{usecase: usecase, logger: logger}
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	result, err := h.usecase.Execute(r.Context(), id)
	if err != nil {
		domainerror.Write(r.Context(), w, err, r.URL.Path, h.logger)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(triphttp.ToAPI(result)); err != nil {
		h.logger.ErrorContext(r.Context(), "write finish trip response", "error", err)
	}
}
