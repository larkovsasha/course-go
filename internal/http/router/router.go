package router

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/larkovsasha/course-go/internal/domain/trip"
	api "github.com/larkovsasha/course-go/internal/generated"
	"github.com/larkovsasha/course-go/internal/http/domainerror"
	"github.com/larkovsasha/course-go/internal/http/health"
	createhandler "github.com/larkovsasha/course-go/internal/http/trip/create"
	finishhandler "github.com/larkovsasha/course-go/internal/http/trip/finish"
	gethandler "github.com/larkovsasha/course-go/internal/http/trip/get"
)

type Handler struct {
	create *createhandler.Handler
	get    *gethandler.Handler
	finish *finishhandler.Handler
	health *health.Handler
}

func NewHandler(
	create *createhandler.Handler,
	get *gethandler.Handler,
	finish *finishhandler.Handler,
	health *health.Handler,
) *Handler {
	return &Handler{create: create, get: get, finish: finish, health: health}
}

func (h *Handler) CreateTrip(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	h.create.Handle(w, r, params)
}

func (h *Handler) GetTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	h.get.Handle(w, r, tripID)
}

func (h *Handler) FinishTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	h.finish.Handle(w, r, tripID)
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	h.health.Health(w, r)
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	h.health.Ready(w, r)
}

func New(handler *Handler, logger *slog.Logger) http.Handler {
	baseRouter := chi.NewRouter()
	return api.HandlerWithOptions(handler, api.ChiServerOptions{
		BaseRouter: baseRouter,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, _ error) {
			domainerror.Write(
				r.Context(),
				w,
				trip.ValidationError{Reason: "request validation failed"},
				r.URL.Path,
				logger,
			)
		},
	})
}
