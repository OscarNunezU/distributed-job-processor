package pool

import (
	"context"
	"sync"
	"time"

	"github.com/OscarNunezU/distributed-job-processor/worker/internal/application/registry"
	"github.com/OscarNunezU/distributed-job-processor/worker/internal/domain"
	"github.com/OscarNunezU/distributed-job-processor/worker/internal/infrastructure/logger"
	"github.com/OscarNunezU/distributed-job-processor/worker/internal/infrastructure/metrics"
)

type WorkerPool struct {
	concurrency int
	timeout     time.Duration
	registry    *registry.HandlerRegistry
	repo        domain.JobRepository
	metrics     *metrics.Metrics
	log         *logger.Logger
}

func New(
	concurrency int,
	timeout time.Duration,
	reg *registry.HandlerRegistry,
	repo domain.JobRepository,
	m *metrics.Metrics,
	log *logger.Logger,
) *WorkerPool {
	return &WorkerPool{
		concurrency: concurrency,
		timeout:     timeout,
		registry:    reg,
		repo:        repo,
		metrics:     m,
		log:         log,
	}
}

func (wp *WorkerPool) Run(ctx context.Context, messages <-chan domain.JobMessage, consumer domain.MessageConsumer) {
	sem := make(chan struct{}, wp.concurrency)
	var wg sync.WaitGroup

	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return
		case msg, ok := <-messages:
			if !ok {
				wg.Wait()
				return
			}

			sem <- struct{}{}
			wg.Add(1)
			wp.metrics.JobStarted()
			go func(m domain.JobMessage) {
				defer wg.Done()
				defer func() { <-sem }()
				defer wp.metrics.JobFinished()
				wp.process(ctx, m, consumer)
			}(msg)
		}
	}
}

func (wp *WorkerPool) process(ctx context.Context, msg domain.JobMessage, consumer domain.MessageConsumer) {
	job := msg.Job
	log := wp.log.With("job_id", job.ID, "job_type", job.Type, "attempt", job.Attempts+1)

	jobCtx, cancel := context.WithTimeout(ctx, wp.timeout)
	defer cancel()

	if err := wp.repo.IncrementAttempts(jobCtx, job.ID); err != nil {
		log.Error("failed to increment attempts", "error", err)
	}
	job.Attempts++

	if err := wp.repo.UpdateStatus(jobCtx, job.ID, domain.StatusProcessing, ""); err != nil {
		log.Error("failed to update status to processing", "error", err)
	}

	handler, err := wp.registry.Get(job.Type)
	if err != nil {
		log.Error("no handler found", "error", err)
		wp.fail(ctx, msg, consumer, job, err.Error(), false)
		return
	}

	err = handler.Handle(jobCtx, job)

	if err != nil {
		log.Error("job failed", "error", err)
		requeue := job.CanRetry()
		wp.fail(ctx, msg, consumer, job, err.Error(), requeue)
		return
	}

	log.Info("job completed")

	if err := wp.repo.UpdateStatus(ctx, job.ID, domain.StatusCompleted, ""); err != nil {
		log.Error("failed to update status to completed", "error", err)
	}
	_ = consumer.Ack(ctx, msg)
}

func (wp *WorkerPool) fail(ctx context.Context, msg domain.JobMessage, consumer domain.MessageConsumer, job *domain.Job, errMsg string, requeue bool) {
	if err := wp.repo.UpdateStatus(ctx, job.ID, domain.StatusFailed, errMsg); err != nil {
		wp.log.Error("failed to update status to failed", "job_id", job.ID, "error", err)
	}
	_ = consumer.Nack(ctx, msg, requeue)
}
