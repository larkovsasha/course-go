package main

import (
	"context"
	"errors"
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
	defer pool.Close()

	tripRepo := repository.NewTripRepository(pool, cfg.DatabaseQueryTimeout)
	historyRepo := repository.NewTripStatusHistoryRepository(pool, cfg.DatabaseQueryTimeout)
	txManager := postgres.NewTransactionManager(pool, cfg.DatabaseQueryTimeout)
	tripService := service.NewTripService(tripRepo, historyRepo, txManager)
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

	ch := make(chan error, 1)
	go func() {
		ch <- server.ListenAndServe()
	}()

	select {
	case err := <-ch:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			pool.Close()
			stop()
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

		pool.Close()
		timer.Stop()
	}

}
