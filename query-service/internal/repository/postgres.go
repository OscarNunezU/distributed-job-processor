package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

type Job struct {
	ID          string
	Type        string
	Status      string
	Attempts    int32
	MaxAttempts int32
	CreatedAt   time.Time
}

type PostgresJobRepository struct {
	db           *sql.DB
	stmtFindByID *sql.Stmt
	stmtFindAll  *sql.Stmt
}

func New(dsn string) (*PostgresJobRepository, error) {
	db, err := connectWithRetry(dsn)
	if err != nil {
		return nil, err
	}

	stmtFindByID, err := db.Prepare(
		`SELECT id, type, status, attempts, max_attempts, created_at FROM jobs WHERE id = $1`,
	)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("prepare findByID: %w", err)
	}

	stmtFindAll, err := db.Prepare(
		`SELECT id, type, status, attempts, max_attempts, created_at FROM jobs ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
	)
	if err != nil {
		stmtFindByID.Close()
		db.Close()
		return nil, fmt.Errorf("prepare findAll: %w", err)
	}

	return &PostgresJobRepository{db: db, stmtFindByID: stmtFindByID, stmtFindAll: stmtFindAll}, nil
}

func (r *PostgresJobRepository) FindByID(ctx context.Context, id string) (*Job, error) {
	row := r.stmtFindByID.QueryRowContext(ctx, id)
	j := &Job{}
	err := row.Scan(&j.ID, &j.Type, &j.Status, &j.Attempts, &j.MaxAttempts, &j.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("findByID: %w", err)
	}
	return j, nil
}

func (r *PostgresJobRepository) FindAll(ctx context.Context, limit, offset int32) ([]*Job, int32, error) {
	rows, err := r.stmtFindAll.QueryContext(ctx, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("findAll: %w", err)
	}
	defer rows.Close()

	var jobs []*Job
	for rows.Next() {
		j := &Job{}
		if err := rows.Scan(&j.ID, &j.Type, &j.Status, &j.Attempts, &j.MaxAttempts, &j.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan: %w", err)
		}
		jobs = append(jobs, j)
	}

	var total int32
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM jobs`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count: %w", err)
	}

	return jobs, total, nil
}

func (r *PostgresJobRepository) Close() {
	r.stmtFindByID.Close()
	r.stmtFindAll.Close()
	r.db.Close()
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
