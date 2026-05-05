package repository

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
	"github.com/OscarNunezU/distributed-job-processor/worker/internal/domain"
)

type PostgresJobRepository struct {
	db *sql.DB
}

func NewPostgresJobRepository(dsn string) (*PostgresJobRepository, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres open: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("postgres ping: %w", err)
	}
	return &PostgresJobRepository{db: db}, nil
}

func (r *PostgresJobRepository) UpdateStatus(ctx context.Context, jobID string, status domain.JobStatus, errMsg string) error {
	const q = `
		UPDATE jobs
		SET status = $1, error = $2, updated_at = NOW()
		WHERE id = $3
	`
	_, err := r.db.ExecContext(ctx, q, string(status), errMsg, jobID)
	if err != nil {
		return fmt.Errorf("update job status: %w", err)
	}
	return nil
}

func (r *PostgresJobRepository) IncrementAttempts(ctx context.Context, jobID string) error {
	const q = `UPDATE jobs SET attempts = attempts + 1, updated_at = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, q, jobID)
	if err != nil {
		return fmt.Errorf("increment attempts: %w", err)
	}
	return nil
}

func (r *PostgresJobRepository) Close() error {
	return r.db.Close()
}
