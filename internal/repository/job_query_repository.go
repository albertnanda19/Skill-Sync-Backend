package repository

import (
	"context"

	"skill-sync/internal/database"

	"github.com/google/uuid"
)

type JobQueryRepository interface {
	CountJobs(ctx context.Context) (int, error)
	CountJobSkills(ctx context.Context) (int, error)
	ListJobIDsWithSkills(ctx context.Context, limit, offset int) ([]uuid.UUID, error)
}

type PostgresJobQueryRepository struct {
	db database.DB
}

func NewPostgresJobQueryRepository(db database.DB) *PostgresJobQueryRepository {
	return &PostgresJobQueryRepository{db: db}
}

func (r *PostgresJobQueryRepository) CountJobs(ctx context.Context) (int, error) {
	row := r.db.QueryRow(ctx, `SELECT COUNT(1) FROM jobs`)
	var c int
	if err := row.Scan(&c); err != nil {
		return 0, err
	}
	return c, nil
}

func (r *PostgresJobQueryRepository) CountJobSkills(ctx context.Context) (int, error) {
	row := r.db.QueryRow(ctx, `SELECT COUNT(1) FROM job_skills`)
	var c int
	if err := row.Scan(&c); err != nil {
		return 0, err
	}
	return c, nil
}

func (r *PostgresJobQueryRepository) ListJobIDsWithSkills(ctx context.Context, limit, offset int) ([]uuid.UUID, error) {
	if limit <= 0 {
		limit = 200
	}
	if limit > 5000 {
		limit = 5000
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := r.db.Query(ctx,
		`SELECT DISTINCT js.job_id
		 FROM job_skills js
		 JOIN jobs j ON j.id = js.job_id
		 WHERE j.is_active = true
		 ORDER BY js.job_id ASC
		 LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
