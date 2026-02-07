package usecase

import (
	"context"
	"strings"

	"skill-sync/internal/repository"

	"github.com/google/uuid"
)

type SkillItem struct {
	ID   uuid.UUID
	Name string
}

type SkillUsecase interface {
	ListSkills(ctx context.Context) ([]SkillItem, error)
	AddSkill(ctx context.Context, name string) (SkillItem, error)
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

func (u *Skill) AddSkill(ctx context.Context, name string) (SkillItem, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return SkillItem{}, ErrInvalidInput
	}

	created, err := u.repo.CreateSkill(ctx, name)
	if err != nil {
		return SkillItem{}, ErrInternal
	}
	return SkillItem{ID: created.ID, Name: created.Name}, nil
}
