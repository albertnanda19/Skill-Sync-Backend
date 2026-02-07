package usecase

import (
	"context"

	"skill-sync/internal/repository"

	"github.com/google/uuid"
)

type SkillItem struct {
	ID   uuid.UUID
	Name string
}

type SkillUsecase interface {
	ListSkills(ctx context.Context) ([]SkillItem, error)
}

type Skill struct {
	repo repository.SkillRepository
}

func NewSkillUsecase(repo repository.SkillRepository) *Skill {
	return &Skill{repo: repo}
}

func (u *Skill) ListSkills(ctx context.Context) ([]SkillItem, error) {
	items, err := u.repo.GetAllSkills(ctx)
	if err != nil {
		return nil, ErrInternal
	}

	out := make([]SkillItem, 0, len(items))
	for _, it := range items {
		out = append(out, SkillItem{ID: it.ID, Name: it.Name})
	}
	return out, nil
}
