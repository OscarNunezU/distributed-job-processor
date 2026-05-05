package registry_test

import (
	"context"
	"errors"
	"testing"

	"github.com/OscarNunezU/distributed-job-processor/worker/internal/application/registry"
	"github.com/OscarNunezU/distributed-job-processor/worker/internal/domain"
)

type mockHandler struct{ called bool }

func (m *mockHandler) Handle(_ context.Context, _ *domain.Job) error {
	m.called = true
	return nil
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	reg := registry.New()
	h := &mockHandler{}
	reg.Register("email", h)

	got, err := reg.Get("email")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != h {
		t.Fatal("expected the registered handler to be returned")
	}
}

func TestRegistry_GetUnknownType(t *testing.T) {
	reg := registry.New()
	_, err := reg.Get("unknown")
	if err == nil {
		t.Fatal("expected error for unknown job type")
	}
}

func TestRegistry_OverwriteHandler(t *testing.T) {
	reg := registry.New()
	h1 := &mockHandler{}
	h2 := &mockHandler{}
	reg.Register("email", h1)
	reg.Register("email", h2)

	got, _ := reg.Get("email")
	if got != h2 {
		t.Fatal("expected the last registered handler to win")
	}
}

func TestDomain_CanRetry(t *testing.T) {
	job := &domain.Job{Attempts: 2, MaxAttempts: 3}
	if !job.CanRetry() {
		t.Fatal("expected CanRetry to be true when attempts < maxAttempts")
	}

	job.Attempts = 3
	if job.CanRetry() {
		t.Fatal("expected CanRetry to be false when attempts == maxAttempts")
	}
}

func TestDomain_IsTerminal(t *testing.T) {
	cases := []struct {
		status   domain.JobStatus
		terminal bool
	}{
		{domain.StatusCompleted, true},
		{domain.StatusFailed, true},
		{domain.StatusProcessing, false},
		{domain.StatusPending, false},
	}

	for _, c := range cases {
		job := &domain.Job{Status: c.status}
		if job.IsTerminal() != c.terminal {
			t.Errorf("status %q: expected IsTerminal=%v", c.status, c.terminal)
		}
	}
}

var _ = errors.New // keep import used
