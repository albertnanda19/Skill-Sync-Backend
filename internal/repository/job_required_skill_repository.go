package repository

import (
	"context"
	"strings"

	"skill-sync/internal/database"

	"github.com/google/uuid"
)

type JobRequiredSkillUpsert struct {
	SkillID          uuid.UUID
	ImportanceWeight int
	RequiredLevel    *int
	IsMandatory      *bool
	RequiredYears    *int
	SourceVersion    int16
}

type JobRequiredSkillRepository interface {
	LoadSkillsByName(ctx context.Context) (map[string]uuid.UUID, error)
	UpsertForJob(ctx context.Context, jobID uuid.UUID, reqs []JobRequiredSkillUpsert) error
}

type PostgresJobRequiredSkillRepository struct {
	db database.DB
}

func NewPostgresJobRequiredSkillRepository(db database.DB) *PostgresJobRequiredSkillRepository {
	return &PostgresJobRequiredSkillRepository{db: db}
}

func (r *PostgresJobRequiredSkillRepository) LoadSkillsByName(ctx context.Context) (map[string]uuid.UUID, error) {
	rows, err := r.db.Query(ctx, `SELECT id, name FROM skills ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		out[name] = id
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *PostgresJobRequiredSkillRepository) UpsertForJob(ctx context.Context, jobID uuid.UUID, reqs []JobRequiredSkillUpsert) error {
	if jobID == uuid.Nil {
		return nil
	}
	if len(reqs) == 0 {
		return nil
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	for _, it := range reqs {
		if it.SkillID == uuid.Nil {
			continue
		}
		_, err := tx.Exec(ctx,
			`INSERT INTO job_skills (
				id, job_id, skill_id, importance_weight, required_level, is_mandatory, required_years, source_version
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
			ON CONFLICT (job_id, skill_id) DO UPDATE SET
				importance_weight = EXCLUDED.importance_weight,
				required_level = EXCLUDED.required_level,
				is_mandatory = EXCLUDED.is_mandatory,
				required_years = EXCLUDED.required_years,
				source_version = EXCLUDED.source_version`,
			uuid.New(),
			jobID,
			it.SkillID,
			it.ImportanceWeight,
			it.RequiredLevel,
			it.IsMandatory,
			it.RequiredYears,
			it.SourceVersion,
		)
		if err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}
