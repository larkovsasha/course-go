package main

import (
	"context"
	"log"

	"github.com/larkovsasha/course-go/internal/config"
	"github.com/larkovsasha/course-go/internal/postgres"
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

}
