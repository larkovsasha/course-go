package idempotencycleanup

import (
	"context"
	"log/slog"
	"time"
)

type Worker struct {
	repository repository
	interval   time.Duration
	logger     *slog.Logger
}

func New(repository repository, interval time.Duration, logger *slog.Logger) *Worker {
	return &Worker{repository: repository, interval: interval, logger: logger}
}

func (w *Worker) Run(ctx context.Context) {
	w.logger.InfoContext(ctx, "idempotency cleanup started")
	defer w.logger.Info("idempotency cleanup stopped")

	w.cleanup(ctx)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.cleanup(ctx)
		}
	}
}

func (w *Worker) cleanup(ctx context.Context) {
	deleted, err := w.repository.DeleteExpired(ctx)
	if err != nil {
		if ctx.Err() == nil {
			w.logger.ErrorContext(ctx, "delete expired idempotency keys", "error", err)
		}
		return
	}

	if deleted > 0 {
		w.logger.InfoContext(ctx, "expired idempotency keys deleted", "count", deleted)
	}
}
