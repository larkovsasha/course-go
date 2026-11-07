package create_test

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	domaintrip "github.com/larkovsasha/course-go/internal/domain/trip"
	api "github.com/larkovsasha/course-go/internal/generated"
	createhandler "github.com/larkovsasha/course-go/internal/http/trip/create"
)

func TestHandlerSuccess(t *testing.T) {
	controller := gomock.NewController(t)
	usecase := NewMockusecase(controller)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := createhandler.NewHandler(usecase, logger)
	id := uuid.New()

	usecase.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(domaintrip.Trip{
		ID:        id,
		UserID:    uuid.New(),
		DriverID:  uuid.New(),
		Status:    domaintrip.Active,
		StartedAt: time.Now(),
	}, false, nil)

	body := []byte(`{
		"user_id":"5cb72c04-7650-45c9-a79b-bcdba0631e0c",
		"driver_id":"8860b315-ec86-42eb-a17c-7c163d721ff5",
		"start_point":{"latitude":59.9398,"longitude":30.3146},
		"end_point":{"latitude":59.9290,"longitude":30.3626},
		"price":1450
	}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/trips", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	handler.Handle(recorder, request, api.CreateTripParams{})

	if recorder.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusCreated)
	}
	if recorder.Header().Get("Location") != "/api/v1/trips/"+id.String() {
		t.Errorf("Location = %s", recorder.Header().Get("Location"))
	}
}

func TestHandlerInvalidBody(t *testing.T) {
	controller := gomock.NewController(t)
	usecase := NewMockusecase(controller)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := createhandler.NewHandler(usecase, logger)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/trips", bytes.NewBufferString(`{"price":-1}`))
	recorder := httptest.NewRecorder()

	handler.Handle(recorder, request, api.CreateTripParams{})

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}
