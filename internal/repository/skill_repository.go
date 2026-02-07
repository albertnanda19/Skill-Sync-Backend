package repository

import (
	"context"

	"skill-sync/internal/database"

	"github.com/google/uuid"
)

type Skill struct {
	ID   uuid.UUID
	Name string
}

type SkillRepository interface {
	GetAllSkills(ctx context.Context) ([]Skill, error)
	CreateSkill(ctx context.Context, name string) (Skill, error)
}

type PostgresSkillRepository struct {
	db database.DB
}

func NewPostgresSkillRepository(db database.DB) *PostgresSkillRepository {
	return &PostgresSkillRepository{db: db}
}

func (r *PostgresSkillRepository) GetAllSkills(ctx context.Context) ([]Skill, error) {
	rows, err := r.db.Query(ctx, `SELECT id, name FROM skills ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Skill, 0)
	for rows.Next() {
		var s Skill
		if err := rows.Scan(&s.ID, &s.Name); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *PostgresSkillRepository) CreateSkill(ctx context.Context, name string) (Skill, error) {
	id := uuid.New()
	_, err := r.db.Exec(ctx, `INSERT INTO skills (id, name) VALUES ($1, $2)`, id, name)
	if err != nil {
		return Skill{}, err
	}
	return Skill{ID: id, Name: name}, nil
}
