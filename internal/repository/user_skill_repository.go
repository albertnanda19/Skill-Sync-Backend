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
	ErrUserSkillNotFound  = errors.New("skill not found")
	ErrUserSkillForbidden = errors.New("forbidden")
)

type UserSkill struct {
	ID               uuid.UUID
	UserID           uuid.UUID
	SkillID          uuid.UUID
	SkillName        string
	ProficiencyLevel int
	YearsExperience  int
}

type UserSkillRepository interface {
	FindByUserID(ctx context.Context, userID uuid.UUID) ([]UserSkill, error)
	FindByUserAndSkill(ctx context.Context, userID uuid.UUID, skillID uuid.UUID) (UserSkill, error)
	SkillExistsByID(ctx context.Context, skillID uuid.UUID) (bool, error)
	Create(ctx context.Context, us UserSkill) (UserSkill, error)
	Update(ctx context.Context, us UserSkill) (UserSkill, error)
	Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
	DeleteUserSkill(ctx context.Context, userID uuid.UUID, skillID uuid.UUID) error
}

type PostgresUserSkillRepository struct {
	db database.DB
}

func NewPostgresUserSkillRepository(db database.DB) *PostgresUserSkillRepository {
	return &PostgresUserSkillRepository{db: db}
}

func (r *PostgresUserSkillRepository) FindByUserID(ctx context.Context, userID uuid.UUID) ([]UserSkill, error) {
	rows, err := r.db.Query(ctx,
		`SELECT us.id, us.user_id, us.skill_id, s.name, COALESCE(us.proficiency_level, 0), COALESCE(us.years_experience, 0)
		 FROM user_skills us
		 JOIN skills s ON s.id = us.skill_id
		 WHERE us.user_id = $1
		 ORDER BY s.name ASC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]UserSkill, 0)
	for rows.Next() {
		var us UserSkill
		if err := rows.Scan(&us.ID, &us.UserID, &us.SkillID, &us.SkillName, &us.ProficiencyLevel, &us.YearsExperience); err != nil {
			return nil, err
		}
		out = append(out, us)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *PostgresUserSkillRepository) FindByUserAndSkill(ctx context.Context, userID uuid.UUID, skillID uuid.UUID) (UserSkill, error) {
	row := r.db.QueryRow(ctx,
		`SELECT us.id, us.user_id, us.skill_id, s.name, COALESCE(us.proficiency_level, 0), COALESCE(us.years_experience, 0)
		 FROM user_skills us
		 JOIN skills s ON s.id = us.skill_id
		 WHERE us.user_id = $1 AND us.skill_id = $2`,
		userID, skillID,
	)

	var us UserSkill
	if err := row.Scan(&us.ID, &us.UserID, &us.SkillID, &us.SkillName, &us.ProficiencyLevel, &us.YearsExperience); err != nil {
		if err == sql.ErrNoRows || errors.Is(err, pgx.ErrNoRows) {
			return UserSkill{}, ErrUserSkillNotFound
		}
		return UserSkill{}, err
	}
	return us, nil
}

func (r *PostgresUserSkillRepository) SkillExistsByID(ctx context.Context, skillID uuid.UUID) (bool, error) {
	var exists bool
	row := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM skills WHERE id = $1)`, skillID)
	if err := row.Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (r *PostgresUserSkillRepository) Create(ctx context.Context, us UserSkill) (UserSkill, error) {
	_, err := r.db.Exec(ctx,
		`INSERT INTO user_skills (id, user_id, skill_id, proficiency_level, years_experience)
		 VALUES ($1, $2, $3, $4, $5)`,
		us.ID, us.UserID, us.SkillID, us.ProficiencyLevel, us.YearsExperience,
	)
	if err != nil {
		return UserSkill{}, err
	}

	row := r.db.QueryRow(ctx,
		`SELECT us.id, us.user_id, us.skill_id, s.name, COALESCE(us.proficiency_level, 0), COALESCE(us.years_experience, 0)
		 FROM user_skills us
		 JOIN skills s ON s.id = us.skill_id
		 WHERE us.id = $1 AND us.user_id = $2`,
		us.ID, us.UserID,
	)

	var created UserSkill
	if err := row.Scan(&created.ID, &created.UserID, &created.SkillID, &created.SkillName, &created.ProficiencyLevel, &created.YearsExperience); err != nil {
		return UserSkill{}, err
	}
	return created, nil
}

func (r *PostgresUserSkillRepository) Update(ctx context.Context, us UserSkill) (UserSkill, error) {
	rowsAffected, err := r.db.Exec(ctx,
		`UPDATE user_skills
		 SET proficiency_level = $1, years_experience = $2
		 WHERE id = $3 AND user_id = $4`,
		us.ProficiencyLevel, us.YearsExperience, us.ID, us.UserID,
	)
	if err != nil {
		return UserSkill{}, err
	}
	if rowsAffected == 0 {
		return UserSkill{}, ErrUserSkillNotFound
	}

	row := r.db.QueryRow(ctx,
		`SELECT us.id, us.user_id, us.skill_id, s.name, COALESCE(us.proficiency_level, 0), COALESCE(us.years_experience, 0)
		 FROM user_skills us
		 JOIN skills s ON s.id = us.skill_id
		 WHERE us.id = $1 AND us.user_id = $2`,
		us.ID, us.UserID,
	)

	var updated UserSkill
	if err := row.Scan(&updated.ID, &updated.UserID, &updated.SkillID, &updated.SkillName, &updated.ProficiencyLevel, &updated.YearsExperience); err != nil {
		if err == sql.ErrNoRows || errors.Is(err, pgx.ErrNoRows) {
			return UserSkill{}, ErrUserSkillNotFound
		}
		return UserSkill{}, err
	}
	return updated, nil
}

func (r *PostgresUserSkillRepository) Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	var owner uuid.UUID
	row := r.db.QueryRow(ctx, `SELECT user_id FROM user_skills WHERE id = $1`, id)
	if err := row.Scan(&owner); err != nil {
		if err == sql.ErrNoRows || errors.Is(err, pgx.ErrNoRows) {
			return ErrUserSkillNotFound
		}
		return err
	}
	if owner != userID {
		return ErrUserSkillForbidden
	}

	_, err := r.db.Exec(ctx, `DELETE FROM user_skills WHERE id = $1`, id)
	if err != nil {
		return err
	}
	return nil
}

func (r *PostgresUserSkillRepository) DeleteUserSkill(ctx context.Context, userID uuid.UUID, skillID uuid.UUID) error {
	exists, err := r.SkillExistsByID(ctx, skillID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrUserSkillNotFound
	}

	rowsAffected, err := r.db.Exec(ctx,
		`DELETE FROM user_skills WHERE user_id = $1 AND skill_id = $2`,
		userID, skillID,
	)
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrUserSkillForbidden
	}
	return nil
}
