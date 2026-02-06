package repository

import (
	"context"
	"database/sql"

	"skill-sync/internal/database"

	"github.com/google/uuid"
)

type JobSkillRequirementV2 struct {
	SkillID          uuid.UUID
	SkillName        string
	RequiredLevel    *int
	IsMandatory      *bool
	RequiredYears    *int
	ImportanceWeight int
}

type JobSkillV2Repository interface {
	FindByJobIDV2(ctx context.Context, jobID uuid.UUID) ([]JobSkillRequirementV2, error)
}

type PostgresJobSkillV2Repository struct {
	db database.DB
}

func NewPostgresJobSkillV2Repository(db database.DB) *PostgresJobSkillV2Repository {
	return &PostgresJobSkillV2Repository{db: db}
}

func (r *PostgresJobSkillV2Repository) FindByJobIDV2(ctx context.Context, jobID uuid.UUID) ([]JobSkillRequirementV2, error) {
	rows, err := r.db.Query(ctx,
		`SELECT js.skill_id,
		        s.name,
		        js.required_level,
		        js.is_mandatory,
		        js.required_years,
		        COALESCE(js.importance_weight, 0)
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

	out := make([]JobSkillRequirementV2, 0)
	for rows.Next() {
		var it JobSkillRequirementV2
		var requiredLevel sql.NullInt32
		var isMandatory sql.NullBool
		var requiredYears sql.NullInt32
		if err := rows.Scan(&it.SkillID, &it.SkillName, &requiredLevel, &isMandatory, &requiredYears, &it.ImportanceWeight); err != nil {
			return nil, err
		}
		if requiredLevel.Valid {
			v := int(requiredLevel.Int32)
			it.RequiredLevel = &v
		}
		if isMandatory.Valid {
			v := isMandatory.Bool
			it.IsMandatory = &v
		}
		if requiredYears.Valid {
			v := int(requiredYears.Int32)
			it.RequiredYears = &v
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
