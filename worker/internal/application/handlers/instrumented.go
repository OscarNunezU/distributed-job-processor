package handlers

import (
	"context"
	"time"

	"github.com/OscarNunezU/distributed-job-processor/worker/internal/domain"
	"github.com/OscarNunezU/distributed-job-processor/worker/internal/infrastructure/metrics"
)

// InstrumentedHandler wraps any JobHandler and records Prometheus metrics for
// every execution. It satisfies domain.JobHandler so it is transparent to callers.
//
// Stack additional decorators by wrapping further:
//
//	NewInstrumented(NewTraced(NewEmailHandler(log), tracer), "email", m)
type InstrumentedHandler struct {
	inner   domain.JobHandler
	jobType string
	metrics *metrics.Metrics
}

func NewInstrumented(inner domain.JobHandler, jobType string, m *metrics.Metrics) *InstrumentedHandler {
	return &InstrumentedHandler{inner: inner, jobType: jobType, metrics: m}
}

func (h *InstrumentedHandler) Handle(ctx context.Context, job *domain.Job) error {
	start := time.Now()
	err := h.inner.Handle(ctx, job)
	duration := time.Since(start).Seconds()

	if err != nil {
		h.metrics.JobFailed(h.jobType)
		return err
	}

	h.metrics.JobCompleted(h.jobType, duration)
	return nil
}
