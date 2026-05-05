package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/OscarNunezU/distributed-job-processor/worker/internal/domain"
	"github.com/OscarNunezU/distributed-job-processor/worker/internal/infrastructure/logger"
)

type EmailHandler struct {
	log *logger.Logger
}

func NewEmailHandler(log *logger.Logger) *EmailHandler {
	return &EmailHandler{log: log}
}

func (h *EmailHandler) Handle(ctx context.Context, job *domain.Job) error {
	to, ok := job.Payload["to"].(string)
	if !ok || to == "" {
		return fmt.Errorf("missing required payload field: to")
	}
	subject, _ := job.Payload["subject"].(string)

	h.log.Info("sending email", "job_id", job.ID, "to", to, "subject", subject)

	// Simulate work
	select {
	case <-time.After(200 * time.Millisecond):
	case <-ctx.Done():
		return ctx.Err()
	}

	h.log.Info("email sent", "job_id", job.ID, "to", to)
	return nil
}
