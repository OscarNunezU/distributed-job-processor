package handlers

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/OscarNunezU/distributed-job-processor/worker/internal/domain"
	"github.com/OscarNunezU/distributed-job-processor/worker/internal/infrastructure/metrics"
)

// InstrumentedHandler wraps any JobHandler, records Prometheus metrics, and
// creates an OTel child span so job processing is visible in distributed traces.
type InstrumentedHandler struct {
	inner   domain.JobHandler
	jobType string
	metrics *metrics.Metrics
}

func NewInstrumented(inner domain.JobHandler, jobType string, m *metrics.Metrics) *InstrumentedHandler {
	return &InstrumentedHandler{inner: inner, jobType: jobType, metrics: m}
}

func (h *InstrumentedHandler) Handle(ctx context.Context, job *domain.Job) error {
	ctx, span := otel.Tracer("worker").Start(ctx, "job.process",
		trace.WithAttributes(
			attribute.String("job.id", job.ID),
			attribute.String("job.type", h.jobType),
		),
	)
	defer span.End()

	start := time.Now()
	err := h.inner.Handle(ctx, job)
	duration := time.Since(start).Seconds()

	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		h.metrics.JobFailed(h.jobType)
		return err
	}

	h.metrics.JobCompleted(h.jobType, duration)
	return nil
}
