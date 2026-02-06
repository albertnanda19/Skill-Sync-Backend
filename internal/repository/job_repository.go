package repository

import (
	"context"
	"database/sql"
	"errors"

	"skill-sync/internal/database"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrJobNotFound = errors.New("job not found")
)

type JobRepository interface {
	ExistsByID(ctx context.Context, jobID uuid.UUID) (bool, error)
	ListJobs(ctx context.Context, limit, offset int) ([]Job, error)
}

type Job struct {
	ID       uuid.UUID
	Title    string
	Company  string
	Location string
}

type PostgresJobRepository struct {
	db database.DB
}

func NewPostgresJobRepository(db database.DB) *PostgresJobRepository {
	return &PostgresJobRepository{db: db}
}

func (r *PostgresJobRepository) ExistsByID(ctx context.Context, jobID uuid.UUID) (bool, error) {
	var exists bool
	row := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM jobs WHERE id = $1)`, jobID)
	if err := row.Scan(&exists); err != nil {
		if err == sql.ErrNoRows || errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return exists, nil
}

func (r *PostgresJobRepository) ListJobs(ctx context.Context, limit, offset int) ([]Job, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := r.db.Query(ctx,
		`SELECT id, COALESCE(title, ''), COALESCE(company, ''), COALESCE(location, '')
		 FROM jobs
		 ORDER BY created_at DESC
		 LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Job, 0)
	for rows.Next() {
		var j Job
		if err := rows.Scan(&j.ID, &j.Title, &j.Company, &j.Location); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
