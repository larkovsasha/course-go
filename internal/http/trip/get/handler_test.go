package get_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/larkovsasha/course-go/internal/domain/trip"
	gethandler "github.com/larkovsasha/course-go/internal/http/trip/get"
)

func TestHandlerNotFound(t *testing.T) {
	controller := gomock.NewController(t)
	usecase := NewMockusecase(controller)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := gethandler.NewHandler(usecase, logger)
	id := uuid.New()
	usecase.EXPECT().Execute(gomock.Any(), id).Return(trip.Trip{}, trip.ErrTripNotFound)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/trips/"+id.String(), nil)
	recorder := httptest.NewRecorder()
	handler.Handle(recorder, request, id)

	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}
