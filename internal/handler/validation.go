package handler

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	api "github.com/larkovsasha/course-go/internal/generated"
)

func validateCreateTrip(body []byte, request api.CreateTripJSONRequestBody) error {
	var fields map[string]json.RawMessage
	err := json.Unmarshal(body, &fields)
	if err != nil {
		return err
	}

	for _, field := range []string{"price", "user_id", "driver_id", "start_point", "end_point"} {
		if raw, ok := fields[field]; !ok || string(raw) == "null" {
			return fmt.Errorf("Request should contain %s", field)
		}
	}

	for _, pointName := range []string{"start_point", "end_point"} {
		var point map[string]json.RawMessage
		if err := json.Unmarshal(fields[pointName], &point); err != nil {
			return fmt.Errorf("decode %s: %w", pointName, err)
		}

		for _, coordinate := range []string{"latitude", "longitude"} {
			if raw, ok := point[coordinate]; !ok || string(raw) == "null" {
				return fmt.Errorf("Request should contain %s.%s", pointName, coordinate)
			}
		}
	}

	for _, longitude := range []float64{request.StartPoint.Longitude, request.EndPoint.Longitude} {
		if longitude > 180 || longitude < -180 {
			return fmt.Errorf("Wrong value for longitude %f", longitude)
		}
	}

	for _, latitude := range []float64{request.StartPoint.Latitude, request.EndPoint.Latitude} {
		if latitude > 90 || latitude < -90 {
			return fmt.Errorf("Wrong value for latitude %f", latitude)
		}
	}

	if request.DriverId == uuid.Nil {
		return errors.New("Wrong value for driver id")
	}

	if request.UserId == uuid.Nil {
		return errors.New("Wrong value for user id")
	}

	if request.Price < 0 {
		return errors.New("Wrong value for price")
	}

	return nil
}
