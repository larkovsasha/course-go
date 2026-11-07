package health

import "context"

type readiness interface {
	Ping(ctx context.Context) error
}
