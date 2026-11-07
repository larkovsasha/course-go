package health

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	api "github.com/larkovsasha/course-go/internal/generated"
)

type Handler struct {
	readiness readiness
	timeout   time.Duration
	logger    *slog.Logger
}

func NewHandler(readiness readiness, timeout time.Duration, logger *slog.Logger) *Handler {
	return &Handler{readiness: readiness, timeout: timeout, logger: logger}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	h.writeResponse(w, r, http.StatusOK, api.Ok)
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	if err := h.readiness.Ping(ctx); err != nil {
		h.writeResponse(w, r, http.StatusServiceUnavailable, api.Unavailable)
		return
	}

	h.writeResponse(w, r, http.StatusOK, api.Ok)
}

func (h *Handler) writeResponse(
	w http.ResponseWriter,
	r *http.Request,
	status int,
	value api.HealthResponseStatus,
) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(api.HealthResponse{Status: value}); err != nil {
		h.logger.ErrorContext(r.Context(), "write health response", "error", err)
	}
}
