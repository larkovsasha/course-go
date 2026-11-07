package create

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/google/uuid"

	"github.com/larkovsasha/course-go/internal/domain/trip"
)

type request struct {
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

const requiredReason = "must not be null"

func decode(body []byte) (request, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()

	var value request
	if err := decoder.Decode(&value); err != nil {
		return request{}, trip.ValidationError{Reason: "request body contains invalid JSON"}
	}

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return request{}, trip.ValidationError{Reason: "request body must contain exactly one JSON object"}
	}

	return value, nil
}

func validate(value request) error {
	if value.UserID == nil {
		return trip.ValidationError{Field: "user_id", Reason: requiredReason}
	}
	if *value.UserID == uuid.Nil {
		return trip.ValidationError{Field: "user_id", Reason: "must be a non-zero UUID"}
	}
	if value.DriverID == nil {
		return trip.ValidationError{Field: "driver_id", Reason: requiredReason}
	}
	if *value.DriverID == uuid.Nil {
		return trip.ValidationError{Field: "driver_id", Reason: "must be a non-zero UUID"}
	}
	if value.StartPoint == nil {
		return trip.ValidationError{Field: "start_point", Reason: requiredReason}
	}
	if value.EndPoint == nil {
		return trip.ValidationError{Field: "end_point", Reason: requiredReason}
	}
	if value.Price == nil {
		return trip.ValidationError{Field: "price", Reason: requiredReason}
	}
	if *value.Price < 0 {
		return trip.ValidationError{Field: "price", Reason: "must be greater than or equal to 0"}
	}
	if err := validateCoordinates("start_point", value.StartPoint); err != nil {
		return err
	}
	if err := validateCoordinates("end_point", value.EndPoint); err != nil {
		return err
	}

	return nil
}

func validateCoordinates(name string, value *coordinates) error {
	if value.Latitude == nil {
		return trip.ValidationError{Field: name + ".latitude", Reason: requiredReason}
	}
	if *value.Latitude < -90 || *value.Latitude > 90 {
		return trip.ValidationError{Field: name + ".latitude", Reason: "must be between -90 and 90"}
	}
	if value.Longitude == nil {
		return trip.ValidationError{Field: name + ".longitude", Reason: requiredReason}
	}
	if *value.Longitude < -180 || *value.Longitude > 180 {
		return trip.ValidationError{Field: name + ".longitude", Reason: "must be between -180 and 180"}
	}

	return nil
}
