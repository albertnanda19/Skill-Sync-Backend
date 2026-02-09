package usecase

import (
	"context"

	"skill-sync/internal/repository"

	"github.com/google/uuid"
)

type JobSourceItem struct {
	ID      uuid.UUID
	Name    string
	BaseURL string
}

type JobSourceUsecase interface {
	ListJobSources(ctx context.Context) ([]JobSourceItem, error)
}

type JobSource struct {
	repo repository.JobSourceRepository
}

func NewJobSourceUsecase(repo repository.JobSourceRepository) *JobSource {
	return &JobSource{repo: repo}
}

func (u *JobSource) ListJobSources(ctx context.Context) ([]JobSourceItem, error) {
	items, err := u.repo.ListJobSources(ctx)
	if err != nil {
		return nil, ErrInternal
	}
	out := make([]JobSourceItem, 0, len(items))
	for _, it := range items {
		out = append(out, JobSourceItem{ID: it.ID, Name: it.Name, BaseURL: it.BaseURL})
	}
	return out, nil
}

var _ JobSourceUsecase = (*JobSource)(nil)
