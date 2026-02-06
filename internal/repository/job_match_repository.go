package repository

import (
	"context"
	"time"

	"skill-sync/internal/database"

	"github.com/google/uuid"
)

type JobMatchUpsert struct {
	UserID    uuid.UUID
	JobID     uuid.UUID
	Score     float64
	MatchedAt time.Time
}

type JobMatchRepository interface {
	Upsert(ctx context.Context, m JobMatchUpsert) error
}

type PostgresJobMatchRepository struct {
	db database.DB
}

func NewPostgresJobMatchRepository(db database.DB) *PostgresJobMatchRepository {
	return &PostgresJobMatchRepository{db: db}
}

func (r *PostgresJobMatchRepository) Upsert(ctx context.Context, m JobMatchUpsert) error {
	if m.UserID == uuid.Nil || m.JobID == uuid.Nil {
		return nil
	}
	if m.MatchedAt.IsZero() {
		m.MatchedAt = time.Now().UTC()
	}

	_, err := r.db.Exec(ctx,
		`INSERT INTO job_matches (id, user_id, job_id, match_score, matched_at)
		 VALUES ($1,$2,$3,$4,$5)
		 ON CONFLICT (user_id, job_id) DO UPDATE SET
			match_score = EXCLUDED.match_score,
			matched_at = EXCLUDED.matched_at`,
		uuid.New(),
		m.UserID,
		m.JobID,
		m.Score,
		m.MatchedAt,
	)
	return err
}
