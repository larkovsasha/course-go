package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/larkovsasha/course-go/internal/config"
	"github.com/larkovsasha/course-go/internal/handler"
	"github.com/larkovsasha/course-go/internal/postgres"
	"github.com/larkovsasha/course-go/internal/repository"
	"github.com/larkovsasha/course-go/internal/service"
)

func main() {
	cfg, err := config.GetConfig()
	if err != nil {
		log.Fatalf("Failed to read config: %v", err)
	}

	ctx := context.Background()
	pool, err := postgres.GetPool(ctx, &cfg)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}

	tripRepo := repository.NewTripRepository(pool, cfg.DatabaseQueryTimeout)
	historyRepo := repository.NewTripStatusHistoryRepository(pool, cfg.DatabaseQueryTimeout)
	idempotencyRepo := repository.NewIdempotencyRepository(pool, cfg.DatabaseQueryTimeout)
	txManager := postgres.NewTransactionManager(pool, cfg.DatabaseQueryTimeout)
	tripService := service.NewTripService(tripRepo, historyRepo, idempotencyRepo, txManager, cfg.IdempotencyTTL)
	httpHandler := handler.NewHandler(tripService, pool, cfg.DatabaseQueryTimeout)
	router := handler.NewRouter(httpHandler)

	server := http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadTimeout:       cfg.HTTPReadTimeout,
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
	}

	notifyCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cleanupDone := make(chan struct{})
	go runIdempotencyCleanup(
		notifyCtx,
		idempotencyRepo,
		cfg.IdempotencyTTLCleanupInterval,
		cleanupDone,
	)

	ch := make(chan error, 1)
	go func() {
		ch <- server.ListenAndServe()
	}()

	select {
	case err := <-ch:
		stop()
		<-cleanupDone
		pool.Close()
		if err != nil {
			log.Fatalf("HTTP server failed: %v", err)
		}
	case <-notifyCtx.Done():
		timer := time.AfterFunc(cfg.ShutdownTimeout, func() {
			log.Printf("Shutdown timeout exceeded")
			os.Exit(1)
		})

		err = server.Shutdown(ctx)
		if err != nil {
			log.Printf("Failed to stop server: %v", err)
		}

		<-cleanupDone
		pool.Close()
		timer.Stop()
	}

}

func runIdempotencyCleanup(
	ctx context.Context,
	repo *repository.IdempotencyRepository,
	interval time.Duration,
	done chan<- struct{},
) {
	defer close(done)

	cleanup := func() {
		deleted, err := repo.DeleteExpired(ctx)
		if err != nil {
			if ctx.Err() == nil {
				log.Printf("delete expired idempotency keys: %v", err)
			}
			return
		}
		if deleted > 0 {
			log.Printf("deleted expired idempotency keys: %d", deleted)
		}
	}

	cleanup()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if ctx.Err() != nil {
				return
			}
			cleanup()
		}
	}

}
