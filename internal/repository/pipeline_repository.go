package repository

import (
	"context"
	"database/sql"
	"time"

	"skill-sync/internal/database"
	"skill-sync/internal/domain"
)

type PipelineRepository interface {
	GetTotalJobs(ctx context.Context) (int, error)
	GetJobsToday(ctx context.Context) (int, error)
	GetSourceStats(ctx context.Context) ([]domain.SourceStat, error)
}

type PostgresPipelineRepository struct {
	db database.DB
}

func NewPostgresPipelineRepository(db database.DB) *PostgresPipelineRepository {
	return &PostgresPipelineRepository{db: db}
}

func (r *PostgresPipelineRepository) GetTotalJobs(ctx context.Context) (int, error) {
	row := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM jobs`)
	var c int
	if err := row.Scan(&c); err != nil {
		return 0, err
	}
	return c, nil
}

func (r *PostgresPipelineRepository) GetJobsToday(ctx context.Context) (int, error) {
	row := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM jobs WHERE created_at >= CURRENT_DATE`)
	var c int
	if err := row.Scan(&c); err != nil {
		return 0, err
	}
	return c, nil
}

func (r *PostgresPipelineRepository) GetSourceStats(ctx context.Context) ([]domain.SourceStat, error) {
	rows, err := r.db.Query(ctx, `SELECT source, COUNT(*) as total_jobs, MAX(created_at) as last_job_time FROM jobs GROUP BY source ORDER BY total_jobs DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]domain.SourceStat, 0)
	for rows.Next() {
		var src sql.NullString
		var total int
		var last sql.NullTime
		if err := rows.Scan(&src, &total, &last); err != nil {
			return nil, err
		}
		st := domain.SourceStat{Source: "unknown", TotalJobs: total}
		if src.Valid {
			st.Source = src.String
		}
		if last.Valid {
			st.LastJobTime = last.Time.UTC()
		} else {
			st.LastJobTime = time.Time{}
		}
		out = append(out, st)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
