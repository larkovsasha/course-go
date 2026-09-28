package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/larkovsasha/course-go/internal/config"
)

func GetPool(
	ctx context.Context,
	config *config.Config,
) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(config.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	poolConfig.MaxConns = config.DatabaseMaxConns
	poolConfig.MinConns = config.DatabaseMinConns
	poolConfig.MaxConnLifetime = config.DatabaseMaxConnLifetime
	poolConfig.ConnConfig.ConnectTimeout = config.DatabaseConnectTimeout

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("creating pgxpool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, config.DatabaseConnectTimeout)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}
