package main

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	RabbitMQURL string
	PostgresDSN string
	Concurrency int
	JobTimeout  time.Duration
	MetricsPort string
}

func loadConfig() (*Config, error) {
	concurrency, err := strconv.Atoi(getEnv("WORKER_CONCURRENCY", "5"))
	if err != nil {
		return nil, fmt.Errorf("invalid WORKER_CONCURRENCY: %w", err)
	}

	timeoutSecs, err := strconv.Atoi(getEnv("JOB_TIMEOUT_SECONDS", "30"))
	if err != nil {
		return nil, fmt.Errorf("invalid JOB_TIMEOUT_SECONDS: %w", err)
	}

	return &Config{
		RabbitMQURL: mustEnv("RABBITMQ_URL"),
		PostgresDSN: mustEnv("POSTGRES_DSN"),
		Concurrency: concurrency,
		JobTimeout:  time.Duration(timeoutSecs) * time.Second,
		MetricsPort: getEnv("METRICS_PORT", "9090"),
	}, nil
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("required env var %q is not set", key))
	}
	return v
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
