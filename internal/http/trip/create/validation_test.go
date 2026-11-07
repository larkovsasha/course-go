package create

import (
	"testing"

	"github.com/google/uuid"
)

func TestValidate(t *testing.T) {
	userID := uuid.New()
	driverID := uuid.New()
	latitude := 59.9398
	longitude := 30.3146
	price := int64(1450)

	validRequest := func() request {
		return request{
			UserID:   &userID,
			DriverID: &driverID,
			StartPoint: &coordinates{
				Latitude:  &latitude,
				Longitude: &longitude,
			},
			EndPoint: &coordinates{
				Latitude:  &latitude,
				Longitude: &longitude,
			},
			Price: &price,
		}
	}

	tests := []struct {
		name    string
		change  func(*request)
		wantErr bool
	}{
		{name: "valid request", change: func(*request) {}},
		{name: "missing user ID", change: func(value *request) { value.UserID = nil }, wantErr: true},
		{name: "missing driver ID", change: func(value *request) { value.DriverID = nil }, wantErr: true},
		{name: "missing start point", change: func(value *request) { value.StartPoint = nil }, wantErr: true},
		{name: "missing end point", change: func(value *request) { value.EndPoint = nil }, wantErr: true},
		{name: "missing price", change: func(value *request) { value.Price = nil }, wantErr: true},
		{name: "negative price", change: func(value *request) { invalid := int64(-1); value.Price = &invalid }, wantErr: true},
		{name: "invalid latitude", change: func(value *request) { invalid := 91.0; value.StartPoint.Latitude = &invalid }, wantErr: true},
		{name: "invalid longitude", change: func(value *request) { invalid := 181.0; value.EndPoint.Longitude = &invalid }, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := validRequest()
			test.change(&value)

			err := validate(value)
			if !test.wantErr && err != nil {
				t.Fatalf("validate() error = %v", err)
			}
			if test.wantErr && err == nil {
				t.Fatal("validate() error = nil, want validation error")
			}
		})
	}
}

func TestDecodeInvalidBody(t *testing.T) {
	tests := []string{
		`{"user_id":`,
		`{"unknown":true}`,
		`{} {}`,
	}

	for _, body := range tests {
		if _, err := decode([]byte(body)); err == nil {
			t.Errorf("decode(%q) error = nil", body)
		}
	}
}
