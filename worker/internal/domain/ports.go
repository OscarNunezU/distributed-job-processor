package domain

import "context"

// JobRepository is the port for persisting job state.
type JobRepository interface {
	UpdateStatus(ctx context.Context, jobID string, status JobStatus, errMsg string) error
	// IncrementAttempts bumps the DB counter and returns the new value so the
	// worker can evaluate CanRetry() against the authoritative count, not the
	// stale value embedded in the queue message.
	IncrementAttempts(ctx context.Context, jobID string) (int, error)
}

// MessageConsumer is the port for consuming jobs from a broker.
type MessageConsumer interface {
	Consume(ctx context.Context) (<-chan JobMessage, error)
	Ack(ctx context.Context, msg JobMessage) error
	Nack(ctx context.Context, msg JobMessage, requeue bool) error
	Close() error
}

// JobHandler is the port every job type must implement.
type JobHandler interface {
	Handle(ctx context.Context, job *Job) error
}

// JobMessage wraps a raw broker message with its decoded job.
type JobMessage struct {
	Job     *Job
	RawBody []byte
	// DeliveryTag is broker-specific metadata needed to ack/nack.
	DeliveryTag uint64
	// TraceHeaders carries W3C trace context extracted from the broker message
	// so the worker can continue the distributed trace started by the API.
	TraceHeaders map[string]string
}
