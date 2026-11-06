package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/google/uuid"
)

type createTripRequest struct {
	UserID     *uuid.UUID   `json:"user_id"`
	DriverID   *uuid.UUID   `json:"driver_id"`
	StartPoint *coordinates `json:"start_point"`
	EndPoint   *coordinates `json:"end_point"`
	Price      *int64       `json:"price"`
}

type coordinates struct {
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
}

func decodeCreateTrip(body []byte) (createTripRequest, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()

	request := createTripRequest{}
	if err := decoder.Decode(&request); err != nil {
		return createTripRequest{}, fmt.Errorf("decode request body: %w", err)
	}

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return createTripRequest{}, errors.New("request body must contain exactly one JSON object")
		}
		return createTripRequest{}, fmt.Errorf("invalid data after request body: %w", err)
	}

	return request, nil
}

func validateCreateTrip(request createTripRequest) error {
	if request.UserID == nil {
		return errors.New("request must contain a non-null user_id")
	}
	if request.DriverID == nil {
		return errors.New("request must contain a non-null driver_id")
	}
	if request.StartPoint == nil {
		return errors.New("request must contain a non-null start_point")
	}
	if request.EndPoint == nil {
		return errors.New("request must contain a non-null end_point")
	}
	if request.Price == nil {
		return errors.New("request must contain a non-null price")
	}

	if err := validateCoordinates("start_point", request.StartPoint); err != nil {
		return err
	}
	if err := validateCoordinates("end_point", request.EndPoint); err != nil {
		return err
	}

	if *request.DriverID == uuid.Nil {
		return errors.New("driver_id must be a non-zero UUID")
	}
	if *request.UserID == uuid.Nil {
		return errors.New("user_id must be a non-zero UUID")
	}
	if *request.Price < 0 {
		return errors.New("price must be greater than or equal to 0")
	}

	return nil
}

func validateCoordinates(name string, point *coordinates) error {
	if point.Latitude == nil {
		return fmt.Errorf("request must contain a non-null %s.latitude", name)
	}
	if point.Longitude == nil {
		return fmt.Errorf("request must contain a non-null %s.longitude", name)
	}
	if *point.Latitude < -90 || *point.Latitude > 90 {
		return fmt.Errorf("%s.latitude must be between -90 and 90", name)
	}
	if *point.Longitude < -180 || *point.Longitude > 180 {
		return fmt.Errorf("%s.longitude must be between -180 and 180", name)
	}

	return nil
}
