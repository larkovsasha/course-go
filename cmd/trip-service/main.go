package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/larkovsasha/course-go/internal/config"
	"github.com/larkovsasha/course-go/internal/container"
)

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "trip service failed: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.GetConfig()
	if err != nil {
		return fmt.Errorf("get config: %w", err)
	}

	app, err := container.New(context.Background(), &cfg)
	if err != nil {
		return fmt.Errorf("create application: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	return app.Run(ctx)
}
