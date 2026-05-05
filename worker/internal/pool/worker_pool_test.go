package pool_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/OscarNunezU/distributed-job-processor/worker/internal/application/registry"
	"github.com/OscarNunezU/distributed-job-processor/worker/internal/domain"
	"github.com/OscarNunezU/distributed-job-processor/worker/internal/infrastructure/logger"
	"github.com/OscarNunezU/distributed-job-processor/worker/internal/infrastructure/metrics"
	"github.com/OscarNunezU/distributed-job-processor/worker/internal/pool"
)

// --- mocks ---

type mockRepo struct {
	updateCalled int32
	incrCalled   int32
}

func (m *mockRepo) UpdateStatus(_ context.Context, _ string, _ domain.JobStatus, _ string) error {
	atomic.AddInt32(&m.updateCalled, 1)
	return nil
}

func (m *mockRepo) IncrementAttempts(_ context.Context, _ string) error {
	atomic.AddInt32(&m.incrCalled, 1)
	return nil
}

type mockConsumer struct {
	acked  int32
	nacked int32
}

func (m *mockConsumer) Consume(_ context.Context) (<-chan domain.JobMessage, error) { return nil, nil }

func (m *mockConsumer) Ack(_ context.Context, _ domain.JobMessage) error {
	atomic.AddInt32(&m.acked, 1)
	return nil
}

func (m *mockConsumer) Nack(_ context.Context, _ domain.JobMessage, _ bool) error {
	atomic.AddInt32(&m.nacked, 1)
	return nil
}

func (m *mockConsumer) Close() error { return nil }

type successHandler struct{}

func (h *successHandler) Handle(_ context.Context, _ *domain.Job) error { return nil }

type failHandler struct{}

func (h *failHandler) Handle(_ context.Context, _ *domain.Job) error {
	return errors.New("handler error")
}

// --- helpers ---

func newPool(reg *registry.HandlerRegistry, repo domain.JobRepository) *pool.WorkerPool {
	reg2 := prometheus.NewRegistry()
	m := metrics.New(reg2)
	log := logger.New("test")
	return pool.New(3, 5*time.Second, reg, repo, m, log)
}

func makeMsg(jobType string, attempts, maxAttempts int) domain.JobMessage {
	return domain.JobMessage{
		Job: &domain.Job{
			ID:          "job-1",
			Type:        jobType,
			Payload:     map[string]interface{}{},
			Status:      domain.StatusPending,
			Attempts:    attempts,
			MaxAttempts: maxAttempts,
		},
	}
}

// --- tests ---

func TestWorkerPool_SuccessfulJob(t *testing.T) {
	reg := registry.New()
	reg.Register("email", &successHandler{})
	repo := &mockRepo{}
	consumer := &mockConsumer{}
	wp := newPool(reg, repo)

	msgs := make(chan domain.JobMessage, 1)
	msgs <- makeMsg("email", 0, 3)
	close(msgs)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	wp.Run(ctx, msgs, consumer)

	if atomic.LoadInt32(&consumer.acked) != 1 {
		t.Errorf("expected 1 ack, got %d", consumer.acked)
	}
	if atomic.LoadInt32(&consumer.nacked) != 0 {
		t.Errorf("expected 0 nacks, got %d", consumer.nacked)
	}
}

func TestWorkerPool_FailedJobNoRetry(t *testing.T) {
	reg := registry.New()
	reg.Register("email", &failHandler{})
	repo := &mockRepo{}
	consumer := &mockConsumer{}
	wp := newPool(reg, repo)

	msgs := make(chan domain.JobMessage, 1)
	msgs <- makeMsg("email", 3, 3) // attempts == maxAttempts → no requeue
	close(msgs)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	wp.Run(ctx, msgs, consumer)

	if atomic.LoadInt32(&consumer.nacked) != 1 {
		t.Errorf("expected 1 nack, got %d", consumer.nacked)
	}
	if atomic.LoadInt32(&consumer.acked) != 0 {
		t.Errorf("expected 0 acks, got %d", consumer.acked)
	}
}

func TestWorkerPool_UnknownJobType(t *testing.T) {
	reg := registry.New()
	repo := &mockRepo{}
	consumer := &mockConsumer{}
	wp := newPool(reg, repo)

	msgs := make(chan domain.JobMessage, 1)
	msgs <- makeMsg("unknown-type", 0, 3)
	close(msgs)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	wp.Run(ctx, msgs, consumer)

	if atomic.LoadInt32(&consumer.nacked) != 1 {
		t.Errorf("expected 1 nack for unknown type, got %d", consumer.nacked)
	}
}
