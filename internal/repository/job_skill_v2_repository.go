package repository

import (
	"context"
	"database/sql"
	"errors"

	"skill-sync/internal/database"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
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
	out, err := r.findFromJobRequiredSkills(ctx, jobID)
	if err == nil {
		if len(out) > 0 {
			return out, nil
		}
		// No v2 requirements found -> fallback to v1 mapping
		return r.findFromJobSkillsFallback(ctx, jobID)
	}

	// If table doesn't exist yet in this environment, remain compatible by falling back.
	if isUndefinedTable(err) {
		return r.findFromJobSkillsFallback(ctx, jobID)
	}
	return nil, err
}

func (r *PostgresJobSkillV2Repository) findFromJobRequiredSkills(ctx context.Context, jobID uuid.UUID) ([]JobSkillRequirementV2, error) {
	rows, err := r.db.Query(ctx,
		`SELECT jrs.skill_id,
		        s.name,
		        jrs.required_level,
		        jrs.is_mandatory,
		        jrs.required_years,
		        COALESCE(NULLIF(jrs.weight, 0), 1)
		 FROM job_required_skills jrs
		 JOIN skills s ON s.id = jrs.skill_id
		 WHERE jrs.job_id = $1
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

func (r *PostgresJobSkillV2Repository) findFromJobSkillsFallback(ctx context.Context, jobID uuid.UUID) ([]JobSkillRequirementV2, error) {
	rows, err := r.db.Query(ctx,
		`SELECT js.skill_id,
		        s.name,
		        COALESCE(NULLIF(js.importance_weight, 0), 1)
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
		if err := rows.Scan(&it.SkillID, &it.SkillName, &it.ImportanceWeight); err != nil {
			return nil, err
		}

		// v1 -> v2 mapping
		if it.ImportanceWeight <= 0 {
			it.ImportanceWeight = 1
		}
		reqLvl := it.ImportanceWeight
		if reqLvl < 1 {
			reqLvl = 1
		}
		if reqLvl > 5 {
			reqLvl = 5
		}
		it.RequiredLevel = &reqLvl

		isMandatory := it.ImportanceWeight >= 4
		it.IsMandatory = &isMandatory

		requiredYears := 0
		it.RequiredYears = &requiredYears

		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func isUndefinedTable(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "42P01"
	}
	return false
}
