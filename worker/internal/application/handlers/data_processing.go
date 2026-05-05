package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/OscarNunezU/distributed-job-processor/worker/internal/domain"
	"github.com/OscarNunezU/distributed-job-processor/worker/internal/infrastructure/logger"
)

type DataProcessingHandler struct {
	log *logger.Logger
}

func NewDataProcessingHandler(log *logger.Logger) *DataProcessingHandler {
	return &DataProcessingHandler{log: log}
}

func (h *DataProcessingHandler) Handle(ctx context.Context, job *domain.Job) error {
	source, ok := job.Payload["source"].(string)
	if !ok || source == "" {
		return fmt.Errorf("missing required payload field: source")
	}

	h.log.Info("processing data", "job_id", job.ID, "source", source)

	select {
	case <-time.After(300 * time.Millisecond):
	case <-ctx.Done():
		return ctx.Err()
	}

	h.log.Info("data processed", "job_id", job.ID, "source", source)
	return nil
}
