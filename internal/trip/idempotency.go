package trip

import (
	"time"

	"github.com/google/uuid"
)

type IdempotencyRecord struct {
	Key         uuid.UUID
	RequestHash string
	TripID      uuid.UUID
	ExpiresAt   time.Time
}
