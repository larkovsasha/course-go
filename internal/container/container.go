package container

import (
	"context"
	"errors"
	"fmt"
	"net/http"
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
	"github.com/larkovsasha/course-go/internal/infra/telemetry"
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
	telemetry       *telemetry.Telemetry
	tripMetrics     *telemetry.TripMetrics
}

func New(ctx context.Context, cfg *config.Config) (*Container, error) {
	serviceTelemetry, err := telemetry.New(ctx, cfg)
	if err != nil {
		return nil, err
	}

	pgxTracer, err := telemetry.NewPGXTracer(
		serviceTelemetry.TracerProvider,
		serviceTelemetry.MeterProvider,
	)
	if err != nil {
		_ = serviceTelemetry.Shutdown(ctx)
		return nil, err
	}

	pool, err := postgres.NewPool(ctx, cfg, pgxTracer)
	if err != nil {
		_ = serviceTelemetry.Shutdown(ctx)
		return nil, err
	}

	tripRepo := triprepository.NewRepository(pool, cfg.DatabaseQueryTimeout)
	histroyRepo := triprepository.NewHistoryRepository(pool, cfg.DatabaseQueryTimeout)
	idempotencyRepo := triprepository.NewIdempotencyRepository(pool, cfg.DatabaseQueryTimeout)
	txManager := transaction.NewManager(pool, cfg.DatabaseQueryTimeout)
	tripMetrics, err := telemetry.NewTripMetrics(serviceTelemetry.MeterProvider, tripRepo)
	if err != nil {
		pool.Close()
		_ = serviceTelemetry.Shutdown(ctx)
		return nil, err
	}
	httpMetrics, err := telemetry.NewHTTPMetrics(serviceTelemetry.MeterProvider)
	if err != nil {
		_ = tripMetrics.Close()
		pool.Close()
		_ = serviceTelemetry.Shutdown(ctx)
		return nil, err
	}

	createUsecase := createusecase.New(
		tripRepo,
		histroyRepo,
		idempotencyRepo,
		txManager,
		tripMetrics,
		serviceTelemetry.Logger,
		cfg.IdempotencyTTL,
	)
	getUsecase := getusecase.New(tripRepo)
	finishUsecase := finishusecase.New(tripRepo, histroyRepo, txManager, tripMetrics, serviceTelemetry.Logger)

	createHandler := createhandler.NewHandler(createUsecase, serviceTelemetry.Logger)
	getHandler := gethandler.NewHandler(getUsecase, serviceTelemetry.Logger)
	finishHandler := finishhandler.NewHandler(finishUsecase, serviceTelemetry.Logger)
	healthHandler := health.NewHandler(pool, cfg.DatabaseQueryTimeout, serviceTelemetry.Logger)
	httpHandler := router.NewHandler(createHandler, getHandler, finishHandler, healthHandler)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router.New(httpHandler, serviceTelemetry.Logger, httpMetrics),
		ReadTimeout:       cfg.HTTPReadTimeout,
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
	}

	return &Container{
		server: server,
		cleanupWorker: idempotencycleanup.New(
			idempotencyRepo,
			cfg.IdempotencyTTLCleanupInterval,
			serviceTelemetry.Logger,
		),
		closeDatabase:   pool.Close,
		shutdwonTimeout: cfg.ShutdownTimeout,
		telemetry:       serviceTelemetry,
		tripMetrics:     tripMetrics,
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
		c.telemetry.Logger.Info("service started", "address", c.server.Addr)
		serverErr <- c.server.ListenAndServe()
	}()

	var runErr error
	select {
	case err := <-serverErr:
		cancel()
		workers.Wait()
		if !errors.Is(err, http.ErrServerClosed) {
			runErr = fmt.Errorf("run HTTP server: %w", err)
		}
	case <-ctx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.WithoutCancel(ctx), c.shutdwonTimeout)
		defer shutdownCancel()

		shutdownErr := c.server.Shutdown(shutdownCtx)
		cancel()
		workers.Wait()
		if shutdownErr != nil {
			runErr = fmt.Errorf("shutdown HTTP server: %w", shutdownErr)
		}
	}

	c.telemetry.Logger.Info("service stopped")
	telemetryCtx, telemetryCancel := context.WithTimeout(context.WithoutCancel(ctx), c.shutdwonTimeout)
	defer telemetryCancel()

	telemetryErr := errors.Join(c.tripMetrics.Close(), c.telemetry.Shutdown(telemetryCtx))
	if telemetryErr != nil {
		c.telemetry.Logger.Error("stop telemetry", "error", telemetryErr)
	}

	return runErr
}
