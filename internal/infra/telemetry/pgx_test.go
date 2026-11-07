package telemetry

import "testing"

func TestQueryAttributes(t *testing.T) {
	tests := []struct {
		query          string
		wantOperation  string
		wantCollection string
	}{
		{query: "SELECT id FROM trips WHERE id = $1", wantOperation: "SELECT", wantCollection: "trips"},
		{query: "INSERT INTO trip_status_history (trip_id) VALUES ($1)", wantOperation: "INSERT", wantCollection: "trip_status_history"},
		{query: "UPDATE trips SET status = $1", wantOperation: "UPDATE", wantCollection: "trips"},
		{query: "DELETE FROM idempotency_keys WHERE expires_at < NOW()", wantOperation: "DELETE", wantCollection: "idempotency_keys"},
	}

	for _, test := range tests {
		t.Run(test.wantOperation, func(t *testing.T) {
			operation, collection := queryAttributes(test.query)
			if operation != test.wantOperation {
				t.Errorf("operation = %s, want %s", operation, test.wantOperation)
			}
			if collection != test.wantCollection {
				t.Errorf("collection = %s, want %s", collection, test.wantCollection)
			}
		})
	}
}
