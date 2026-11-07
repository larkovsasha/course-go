package container

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/larkovsasha/course-go/internal/config"
	"github.com/larkovsasha/course-go/internal/http/health"
	"github.com/larkovsasha/course-go/internal/http/router"
	createhandler "github.com/larkovsasha/course-go/internal/http/trip/create"
	finishhandler "github.com/larkovsasha/course-go/internal/http/trip/finish"
	gethandler "github.com/larkovsasha/course-go/internal/http/trip/get"
	"github.com/larkovsasha/course-go/internal/infra/postgres"
	"github.com/larkovsasha/course-go/internal/infra/postgres/transaction"
	triprepository "github.com/larkovsasha/course-go/internal/repository/trip"
	createusecase "github.com/larkovsasha/course-go/internal/usecase/trip/create"
	finishusecase "github.com/larkovsasha/course-go/internal/usecase/trip/finish"
	getusecase "github.com/larkovsasha/course-go/internal/usecase/trip/get"
	"github.com/larkovsasha/course-go/internal/worker/idempotencycleanup"
)

type Container struct {
	server          *http.Server
	cleanupWorker   *idempotencycleanup.Worker
	closeDatabase   func()
	shutdwonTimeout time.Duration
	logger          *slog.Logger
}

func New(ctx context.Context, cfg *config.Config) (*Container, error) {
	logLevel := new(slog.LevelVar)
	if err := logLevel.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		return nil, fmt.Errorf("parse log level: %w", err)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})).With(
		"service", "trip-service",
	)

	pool, err := postgres.NewPool(ctx, cfg)
	if err != nil {
		return nil, err
	}

	tripRepo := triprepository.NewRepository(pool, cfg.DatabaseQueryTimeout)
	histroyRepo := triprepository.NewHistoryRepository(pool, cfg.DatabaseQueryTimeout)
	idempotencyRepo := triprepository.NewIdempotencyRepository(pool, cfg.DatabaseQueryTimeout)
	txManager := transaction.NewManager(pool, cfg.DatabaseQueryTimeout)

	createUsecase := createusecase.New(
		tripRepo,
		histroyRepo,
		idempotencyRepo,
		txManager,
		logger,
		cfg.IdempotencyTTL,
	)
	getUsecase := getusecase.New(tripRepo)
	finishUsecase := finishusecase.New(tripRepo, histroyRepo, txManager, logger)

	createHandler := createhandler.NewHandler(createUsecase, logger)
	getHandler := gethandler.NewHandler(getUsecase, logger)
	finishHandler := finishhandler.NewHandler(finishUsecase, logger)
	healthHandler := health.NewHandler(pool, cfg.DatabaseQueryTimeout, logger)
	httpHandler := router.NewHandler(createHandler, getHandler, finishHandler, healthHandler)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router.New(httpHandler, logger),
		ReadTimeout:       cfg.HTTPReadTimeout,
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
	}

	return &Container{
		server:          server,
		cleanupWorker:   idempotencycleanup.New(idempotencyRepo, cfg.IdempotencyTTLCleanupInterval, logger),
		closeDatabase:   pool.Close,
		shutdwonTimeout: cfg.ShutdownTimeout,
		logger:          logger,
	}, nil
}

func (c *Container) Run(ctx context.Context) error {
	appCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer c.closeDatabase()

	var workers sync.WaitGroup
	workers.Add(1)
	go func() {
		defer workers.Done()
		c.cleanupWorker.Run(appCtx)
	}()

	serverErr := make(chan error, 1)
	go func() {
		c.logger.Info("service started", "address", c.server.Addr)
		serverErr <- c.server.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		cancel()
		workers.Wait()
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("run HTTP server: %w", err)
		}
	case <-ctx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.WithoutCancel(ctx), c.shutdwonTimeout)
		defer shutdownCancel()

		shutdownErr := c.server.Shutdown(shutdownCtx)
		cancel()
		workers.Wait()
		if shutdownErr != nil {
			return fmt.Errorf("shutdown HTTP server: %w", shutdownErr)
		}
	}

	c.logger.Info("service stopped")
	return nil
}
