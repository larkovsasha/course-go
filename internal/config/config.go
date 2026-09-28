package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr                      string
	LogLevel                      string
	ShutdownTimeout               time.Duration
	DatabaseURL                   string
	DatabaseMaxConns              int32
	DatabaseMinConns              int32
	DatabaseMaxConnLifetime       time.Duration
	DatabaseConnectTimeout        time.Duration
	DatabaseQueryTimeout          time.Duration
	HTTPReadTimeout               time.Duration
	HTTPReadHeaderTimeout         time.Duration
	HTTPWriteTimeout              time.Duration
	HTTPIdleTimeout               time.Duration
	IdempotencyTTL                time.Duration
	IdempotencyTTLCleanupInterval time.Duration
}

func GetConfig() (Config, error) {
	httpAddr, err := readField("HTTP_ADDR")
	if err != nil {
		return Config{}, err
	}

	logLevel, err := readField("LOG_LEVEL")
	if err != nil {
		return Config{}, err
	}

	shutdownTimeout, err := duration("SHUTDOWN_TIMEOUT")
	if err != nil {
		return Config{}, err
	}

	databaseURL, err := readField("DATABASE_URL")
	if err != nil {
		return Config{}, err
	}

	maxConns, err := amount("DATABASE_MAX_CONNS", 1)
	if err != nil {
		return Config{}, err
	}

	minConns, err := amount("DATABASE_MIN_CONNS", 0)
	if err != nil {
		return Config{}, err
	}

	if minConns > maxConns {
		return Config{}, errors.New("Database min connections have to be less or egual to max connections")
	}

	maxConnLifetime, err := duration("DATABASE_MAX_CONN_LIFETIME")
	if err != nil {
		return Config{}, err
	}

	connectTimeout, err := duration("DATABASE_CONNECT_TIMEOUT")
	if err != nil {
		return Config{}, err
	}

	queryTimeout, err := duration("DATABASE_QUERY_TIMEOUT")
	if err != nil {
		return Config{}, err
	}

	httpReadTimeout, err := duration("HTTP_READ_TIMEOUT")
	if err != nil {
		return Config{}, err
	}

	httpReadHeaderTimeout, err := duration("HTTP_READ_HEADER_TIMEOUT")
	if err != nil {
		return Config{}, err
	}

	httpWriteTimeout, err := duration("HTTP_WRITE_TIMEOUT")
	if err != nil {
		return Config{}, err
	}

	httpIdleTimeout, err := duration("HTTP_IDLE_TIMEOUT")
	if err != nil {
		return Config{}, err
	}

	idempotencyTTL, err := duration("IDEMPOTENCY_TTL")
	if err != nil {
		return Config{}, err
	}

	idempotencyCleanupInterval, err := duration("IDEMPOTENCY_CLEANUP_INTERVAL")
	if err != nil {
		return Config{}, err
	}

	return Config{
		HTTPAddr:                      httpAddr,
		DatabaseURL:                   databaseURL,
		LogLevel:                      logLevel,
		ShutdownTimeout:               shutdownTimeout,
		DatabaseMaxConns:              maxConns,
		DatabaseMinConns:              minConns,
		DatabaseMaxConnLifetime:       maxConnLifetime,
		DatabaseConnectTimeout:        connectTimeout,
		DatabaseQueryTimeout:          queryTimeout,
		HTTPReadTimeout:               httpReadTimeout,
		HTTPReadHeaderTimeout:         httpReadHeaderTimeout,
		HTTPWriteTimeout:              httpWriteTimeout,
		HTTPIdleTimeout:               httpIdleTimeout,
		IdempotencyTTL:                idempotencyTTL,
		IdempotencyTTLCleanupInterval: idempotencyCleanupInterval,
	}, nil
}

func readField(name string) (string, error) {
	val, exists := os.LookupEnv(name)
	if !exists || val == "" {
		return "", fmt.Errorf("Environment variable %s was not provided", name)
	}
	return val, nil
}

func duration(name string) (time.Duration, error) {
	value, err := readField(name)
	if err != nil {
		return 0, err
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("Failed to parse variable %s: %w", name, err)
	}

	if parsed <= 0 {
		return 0, fmt.Errorf("Variable %s have to be greater than zero", name)
	}
	return parsed, nil
}

func amount(name string, lowerbound int32) (int32, error) {
	value, err := readField(name)
	if err != nil {
		return 0, err
	}

	parsed, err := strconv.ParseInt(value, 10, 32)

	if err != nil {
		return 0, fmt.Errorf("Failed to parse variable %s: %w", name, err)
	}

	if int32(parsed) < lowerbound {
		return 0, fmt.Errorf("Variable %s have to be greater or equal than %d", name, lowerbound)
	}

	return int32(parsed), nil
}
