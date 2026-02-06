package repository

import (
	"context"

	"skill-sync/internal/database"

	"github.com/google/uuid"
)

type PipelineScraperSourceSummary struct {
	JobsScraped int
	Errors      int
}

type PipelineSkillExtractionSummary struct {
	JobsProcessed              int
	JobsSkippedDescriptionNull int
	Errors                     int
}

type PipelineMatchingEngineSummary struct {
	TotalJobsMatched         int
	AverageMatchScore        float64
	JobsWithMandatoryMissing int
}

type PipelineUserRecommendationSummary struct {
	UserID          uuid.UUID
	JobsRecommended int
}

type PipelineStatusRepository interface {
	GetScraperSourceSummary(ctx context.Context, sourceName string) (PipelineScraperSourceSummary, error)
	GetGlintsLinkOnlyCount(ctx context.Context) (int, error)
	GetSkillExtractionSummary(ctx context.Context) (PipelineSkillExtractionSummary, error)
	GetMatchingEngineSummary(ctx context.Context) (PipelineMatchingEngineSummary, error)
	ListRecommendationSummaryByUser(ctx context.Context, limit int) ([]PipelineUserRecommendationSummary, error)
}

type PostgresPipelineStatusRepository struct {
	db database.DB
}

func NewPostgresPipelineStatusRepository(db database.DB) *PostgresPipelineStatusRepository {
	return &PostgresPipelineStatusRepository{db: db}
}

func (r *PostgresPipelineStatusRepository) GetScraperSourceSummary(ctx context.Context, sourceName string) (PipelineScraperSourceSummary, error) {
	var out PipelineScraperSourceSummary

	row := r.db.QueryRow(ctx,
		`SELECT COUNT(1)
		 FROM jobs j
		 JOIN job_sources s ON s.id = j.source_id
		 WHERE s.name = $1`,
		sourceName,
	)
	if err := row.Scan(&out.JobsScraped); err != nil {
		return PipelineScraperSourceSummary{}, err
	}

	row = r.db.QueryRow(ctx,
		`SELECT COALESCE(COUNT(1), 0)
		 FROM scrape_logs sl
		 WHERE sl.level = 'error'
		   AND sl.scrape_run_id = (
			 SELECT sr.id
			 FROM scrape_runs sr
			 JOIN job_sources s ON s.id = sr.source_id
			 WHERE s.name = $1
			 ORDER BY sr.started_at DESC NULLS LAST
			 LIMIT 1
		   )`,
		sourceName,
	)
	if err := row.Scan(&out.Errors); err != nil {
		return PipelineScraperSourceSummary{}, err
	}

	return out, nil
}

func (r *PostgresPipelineStatusRepository) GetGlintsLinkOnlyCount(ctx context.Context) (int, error) {
	row := r.db.QueryRow(ctx,
		`SELECT COALESCE(COUNT(1), 0)
		 FROM jobs j
		 JOIN job_sources s ON s.id = j.source_id
		 WHERE s.name = 'Glints'
		   AND (j.description IS NULL OR BTRIM(j.description) = '')`,
	)
	var c int
	if err := row.Scan(&c); err != nil {
		return 0, err
	}
	return c, nil
}

func (r *PostgresPipelineStatusRepository) GetSkillExtractionSummary(ctx context.Context) (PipelineSkillExtractionSummary, error) {
	var out PipelineSkillExtractionSummary

	row := r.db.QueryRow(ctx, `SELECT COALESCE(COUNT(DISTINCT job_id), 0) FROM job_skills`)
	if err := row.Scan(&out.JobsProcessed); err != nil {
		return PipelineSkillExtractionSummary{}, err
	}

	row = r.db.QueryRow(ctx, `SELECT COALESCE(COUNT(1), 0) FROM jobs WHERE description IS NULL OR BTRIM(description) = ''`)
	if err := row.Scan(&out.JobsSkippedDescriptionNull); err != nil {
		return PipelineSkillExtractionSummary{}, err
	}

	out.Errors = 0
	return out, nil
}

func (r *PostgresPipelineStatusRepository) GetMatchingEngineSummary(ctx context.Context) (PipelineMatchingEngineSummary, error) {
	var out PipelineMatchingEngineSummary

	row := r.db.QueryRow(ctx, `SELECT COALESCE(COUNT(1), 0), COALESCE(AVG(match_score), 0) FROM job_matches`)
	if err := row.Scan(&out.TotalJobsMatched, &out.AverageMatchScore); err != nil {
		return PipelineMatchingEngineSummary{}, err
	}

	row = r.db.QueryRow(ctx,
		`SELECT COALESCE(COUNT(1), 0)
		 FROM job_matches jm
		 WHERE EXISTS (
			SELECT 1
			FROM job_skills js
			WHERE js.job_id = jm.job_id
			  AND COALESCE(js.is_mandatory, false) = true
			  AND NOT EXISTS (
				SELECT 1
				FROM user_skills us
				WHERE us.user_id = jm.user_id
				  AND us.skill_id = js.skill_id
			  )
		 )`,
	)
	if err := row.Scan(&out.JobsWithMandatoryMissing); err != nil {
		return PipelineMatchingEngineSummary{}, err
	}

	return out, nil
}

func (r *PostgresPipelineStatusRepository) ListRecommendationSummaryByUser(ctx context.Context, limit int) ([]PipelineUserRecommendationSummary, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	rows, err := r.db.Query(ctx,
		`SELECT user_id, COUNT(1) AS jobs_recommended
		 FROM job_matches
		 GROUP BY user_id
		 ORDER BY jobs_recommended DESC
		 LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]PipelineUserRecommendationSummary, 0)
	for rows.Next() {
		var it PipelineUserRecommendationSummary
		var uid uuid.UUID
		var cnt int
		if err := rows.Scan(&uid, &cnt); err != nil {
			return nil, err
		}
		it.UserID = uid
		it.JobsRecommended = cnt
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
