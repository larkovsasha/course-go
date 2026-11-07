package trip

import "errors"

var (
	ErrTripNotFound           = errors.New("trip not found")
	ErrTripCompleted          = errors.New("trip already completed")
	ErrDriverBusy             = errors.New("driver already has an active trip")
	ErrIdempotencyConflict    = errors.New("idempotency key already exists with different request data")
	ErrIdempotencyKeyNotFound = errors.New("idempotency key does not exist")
)
