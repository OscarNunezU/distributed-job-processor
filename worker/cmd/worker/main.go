package main

import (
	"context"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/OscarNunezU/distributed-job-processor/worker/internal/application/handlers"
	"github.com/OscarNunezU/distributed-job-processor/worker/internal/application/registry"
	"github.com/OscarNunezU/distributed-job-processor/worker/internal/infrastructure/logger"
	"github.com/OscarNunezU/distributed-job-processor/worker/internal/infrastructure/metrics"
	"github.com/OscarNunezU/distributed-job-processor/worker/internal/infrastructure/queue"
	"github.com/OscarNunezU/distributed-job-processor/worker/internal/infrastructure/repository"
	"github.com/OscarNunezU/distributed-job-processor/worker/internal/pool"
)

func main() {
	log := logger.New("worker")

	cfg, err := loadConfig()
	if err != nil {
		log.Error("failed to load config", "error", err)
		panic(err)
	}

	// Infrastructure
	repo, err := repository.NewPostgresJobRepository(cfg.PostgresDSN)
	if err != nil {
		log.Error("failed to connect to postgres", "error", err)
		panic(err)
	}
	defer repo.Close()

	consumer, err := queue.NewRabbitMQConsumer(cfg.RabbitMQURL, log)
	if err != nil {
		log.Error("failed to connect to rabbitmq", "error", err)
		panic(err)
	}
	defer consumer.Close()

	// Metrics
	promReg := prometheus.NewRegistry()
	m := metrics.New(promReg)

	// Handler registry — add new job types here
	reg := registry.New()
	reg.Register("email", handlers.NewInstrumented(handlers.NewEmailHandler(log), "email", m))
	reg.Register("report", handlers.NewInstrumented(handlers.NewReportHandler(log), "report", m))
	reg.Register("data-processing", handlers.NewInstrumented(handlers.NewDataProcessingHandler(log), "data-processing", m))

	// Worker pool
	wp := pool.New(cfg.Concurrency, cfg.JobTimeout, reg, repo, m, log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Metrics server
	mux := http.NewServeMux()
	mux.Handle("/metrics", m.Handler())
	metricsServer := &http.Server{Addr: fmt.Sprintf(":%s", cfg.MetricsPort), Handler: mux}
	go func() {
		log.Info("metrics server started", "port", cfg.MetricsPort)
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("metrics server error", "error", err)
		}
	}()
	defer metricsServer.Shutdown(ctx) //nolint:errcheck

	log.Info("worker started", "concurrency", cfg.Concurrency, "timeout_s", cfg.JobTimeout.Seconds())

	messages, err := consumer.Consume(ctx)
	if err != nil {
		log.Error("failed to start consumer", "error", err)
		panic(err)
	}

	wp.Run(ctx, messages, consumer)
	log.Info("worker stopped gracefully")
}
