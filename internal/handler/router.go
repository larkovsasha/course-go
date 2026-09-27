package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	api "github.com/larkovsasha/course-go/internal/generated"
)

func NewRouter(handler *Handler) http.Handler {
	router := chi.NewRouter()
	return api.HandlerWithOptions(handler, api.ChiServerOptions{
		BaseRouter: router,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, _ error) {
			writeInvalidRequest(r.Context(), w, r.URL.Path)
		},
	})
}
