package repository

import (
	"context"

	"skill-sync/internal/database"

	"github.com/google/uuid"
)

type JobSource struct {
	ID      uuid.UUID
	Name    string
	BaseURL string
}

type JobSourceRepository interface {
	ListJobSources(ctx context.Context) ([]JobSource, error)
}

type PostgresJobSourceRepository struct {
	db database.DB
}

func NewPostgresJobSourceRepository(db database.DB) *PostgresJobSourceRepository {
	return &PostgresJobSourceRepository{db: db}
}

func (r *PostgresJobSourceRepository) ListJobSources(ctx context.Context) ([]JobSource, error) {
	rows, err := r.db.Query(ctx, `SELECT id, COALESCE(name, ''), COALESCE(base_url, '') FROM job_sources ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]JobSource, 0)
	for rows.Next() {
		var it JobSource
		if err := rows.Scan(&it.ID, &it.Name, &it.BaseURL); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		return []JobSource{}, nil
	}
	return out, nil
}

var _ JobSourceRepository = (*PostgresJobSourceRepository)(nil)
