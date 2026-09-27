package trip

import (
	"time"

	"github.com/google/uuid"
)

type StatusChange struct {
	ID         int64
	TripID     uuid.UUID
	FromStatus *Status
	ToStatus   Status
	Reason     *string
	ChangedAt  time.Time
}
