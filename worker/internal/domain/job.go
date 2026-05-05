package domain

import (
	"time"
)

type JobStatus string

const (
	StatusPending    JobStatus = "pending"
	StatusProcessing JobStatus = "processing"
	StatusCompleted  JobStatus = "completed"
	StatusFailed     JobStatus = "failed"
)

type Job struct {
	ID          string
	Type        string
	Payload     map[string]interface{}
	Status      JobStatus
	Attempts    int
	MaxAttempts int
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Error       string
}

func (j *Job) CanRetry() bool {
	return j.Attempts < j.MaxAttempts
}

func (j *Job) IsTerminal() bool {
	return j.Status == StatusCompleted || j.Status == StatusFailed
}
