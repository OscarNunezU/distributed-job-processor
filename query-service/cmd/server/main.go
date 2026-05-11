package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	gen "github.com/OscarNunezU/distributed-job-processor/query-service/gen/job/v1"
	"github.com/OscarNunezU/distributed-job-processor/query-service/internal/repository"
	"github.com/OscarNunezU/distributed-job-processor/query-service/internal/server"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	dsn := requireEnv("POSTGRES_DSN", log)
	port := getEnv("GRPC_PORT", "5000")

	repo, err := repository.New(dsn)
	if err != nil {
		log.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer repo.Close()

	grpcServer := grpc.NewServer()
	gen.RegisterJobServiceServer(grpcServer, server.NewJobServer(repo, log))

	// Reflection allows grpcurl to work without having the proto file locally.
	reflection.Register(grpcServer)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Error("failed to listen", "port", port, "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Info("query-service started", "port", port)
		if err := grpcServer.Serve(lis); err != nil {
			log.Error("grpc server error", "error", err)
		}
	}()

	<-ctx.Done()
	log.Info("shutting down gracefully")
	grpcServer.GracefulStop()
}

func requireEnv(key string, log *slog.Logger) string {
	v := os.Getenv(key)
	if v == "" {
		log.Error("required environment variable not set", "key", key)
		os.Exit(1)
	}
	return v
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
