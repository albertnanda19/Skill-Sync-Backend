package repository

import (
	"context"

	"skill-sync/internal/database"

	"github.com/google/uuid"
)

type JobSkillRequirement struct {
	SkillID          uuid.UUID
	SkillName        string
	ImportanceWeight int
}

type JobSkillRepository interface {
	FindByJobID(ctx context.Context, jobID uuid.UUID) ([]JobSkillRequirement, error)
}

type PostgresJobSkillRepository struct {
	db database.DB
}

func NewPostgresJobSkillRepository(db database.DB) *PostgresJobSkillRepository {
	return &PostgresJobSkillRepository{db: db}
}

func (r *PostgresJobSkillRepository) FindByJobID(ctx context.Context, jobID uuid.UUID) ([]JobSkillRequirement, error) {
	rows, err := r.db.Query(ctx,
		`SELECT js.skill_id, s.name, COALESCE(js.importance_weight, 0)
		 FROM job_skills js
		 JOIN skills s ON s.id = js.skill_id
		 WHERE js.job_id = $1
		 ORDER BY s.name ASC`,
		jobID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]JobSkillRequirement, 0)
	for rows.Next() {
		var it JobSkillRequirement
		if err := rows.Scan(&it.SkillID, &it.SkillName, &it.ImportanceWeight); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
