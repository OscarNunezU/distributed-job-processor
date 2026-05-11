package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"github.com/OscarNunezU/distributed-job-processor/worker/internal/domain"
)

type PostgresJobRepository struct {
	db               *sql.DB
	stmtUpdateStatus *sql.Stmt
	stmtIncrAttempts *sql.Stmt
}

func NewPostgresJobRepository(dsn string) (*PostgresJobRepository, error) {
	db, err := connectWithRetry(dsn)
	if err != nil {
		return nil, err
	}

	stmtUpdate, err := db.Prepare(`
		UPDATE jobs
		SET status = $1, error = $2, updated_at = NOW()
		WHERE id = $3
	`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("prepare update status: %w", err)
	}

	stmtIncr, err := db.Prepare(`UPDATE jobs SET attempts = attempts + 1, updated_at = NOW() WHERE id = $1 RETURNING attempts`)
	if err != nil {
		stmtUpdate.Close()
		db.Close()
		return nil, fmt.Errorf("prepare increment attempts: %w", err)
	}

	return &PostgresJobRepository{
		db:               db,
		stmtUpdateStatus: stmtUpdate,
		stmtIncrAttempts: stmtIncr,
	}, nil
}

func connectWithRetry(dsn string) (*sql.DB, error) {
	const maxAttempts = 10
	delay := time.Second

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		db, err := sql.Open("postgres", dsn)
		if err != nil {
			return nil, fmt.Errorf("postgres open: %w", err)
		}
		if err := db.Ping(); err != nil {
			db.Close()
			if attempt == maxAttempts {
				return nil, fmt.Errorf("postgres ping after %d attempts: %w", maxAttempts, err)
			}
			time.Sleep(delay)
			if delay < 30*time.Second {
				delay *= 2
			}
			continue
		}
		return db, nil
	}
	return nil, fmt.Errorf("unreachable")
}

func (r *PostgresJobRepository) UpdateStatus(ctx context.Context, jobID string, status domain.JobStatus, errMsg string) error {
	_, err := r.stmtUpdateStatus.ExecContext(ctx, string(status), errMsg, jobID)
	if err != nil {
		return fmt.Errorf("update job status: %w", err)
	}
	return nil
}

func (r *PostgresJobRepository) IncrementAttempts(ctx context.Context, jobID string) (int, error) {
	var newAttempts int
	if err := r.stmtIncrAttempts.QueryRowContext(ctx, jobID).Scan(&newAttempts); err != nil {
		return 0, fmt.Errorf("increment attempts: %w", err)
	}
	return newAttempts, nil
}

func (r *PostgresJobRepository) Close() error {
	r.stmtUpdateStatus.Close()
	r.stmtIncrAttempts.Close()
	return r.db.Close()
}
