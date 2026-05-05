package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/OscarNunezU/distributed-job-processor/worker/internal/domain"
	"github.com/OscarNunezU/distributed-job-processor/worker/internal/infrastructure/logger"
)

type ReportHandler struct {
	log *logger.Logger
}

func NewReportHandler(log *logger.Logger) *ReportHandler {
	return &ReportHandler{log: log}
}

func (h *ReportHandler) Handle(ctx context.Context, job *domain.Job) error {
	reportType, ok := job.Payload["report_type"].(string)
	if !ok || reportType == "" {
		return fmt.Errorf("missing required payload field: report_type")
	}

	h.log.Info("generating report", "job_id", job.ID, "report_type", reportType)

	select {
	case <-time.After(500 * time.Millisecond):
	case <-ctx.Done():
		return ctx.Err()
	}

	h.log.Info("report generated", "job_id", job.ID, "report_type", reportType)
	return nil
}
