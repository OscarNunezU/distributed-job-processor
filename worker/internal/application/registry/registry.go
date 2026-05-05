package registry

import (
	"fmt"

	"github.com/OscarNunezU/distributed-job-processor/worker/internal/domain"
)

// HandlerRegistry maps job types to their handlers.
// Adding a new job type = Register() call, no other changes needed.
type HandlerRegistry struct {
	handlers map[string]domain.JobHandler
}

func New() *HandlerRegistry {
	return &HandlerRegistry{handlers: make(map[string]domain.JobHandler)}
}

func (r *HandlerRegistry) Register(jobType string, handler domain.JobHandler) {
	r.handlers[jobType] = handler
}

func (r *HandlerRegistry) Get(jobType string) (domain.JobHandler, error) {
	h, ok := r.handlers[jobType]
	if !ok {
		return nil, fmt.Errorf("no handler registered for job type %q", jobType)
	}
	return h, nil
}
