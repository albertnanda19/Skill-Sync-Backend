package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"skill-sync/internal/repository"

	"github.com/google/uuid"
)

type mockJobRepo struct {
	items []repository.JobListRow
	err   error
}

func (m mockJobRepo) ExistsByID(context.Context, uuid.UUID) (bool, error)          { return false, nil }
func (m mockJobRepo) ListJobs(context.Context, int, int) ([]repository.Job, error) { return nil, nil }
func (m mockJobRepo) ListActiveJobsWithoutSkills(context.Context, int, int) ([]repository.JobForSkillExtraction, error) {
	return nil, nil
}
func (m mockJobRepo) ListJobsForListing(context.Context, repository.JobListFilter) ([]repository.JobListRow, error) {
	return m.items, m.err
}
func (m mockJobRepo) UpsertJobs(context.Context, []repository.JobUpsert) error { return nil }

type mockJobSkillRepo struct {
	m   map[uuid.UUID][]repository.JobSkillRequirement
	err error
}

func (m mockJobSkillRepo) FindByJobID(context.Context, uuid.UUID) ([]repository.JobSkillRequirement, error) {
	return nil, nil
}
func (m mockJobSkillRepo) FindByJobIDs(context.Context, []uuid.UUID) (map[uuid.UUID][]repository.JobSkillRequirement, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.m, nil
}

func TestJobListUsecase_ListJobs_InvalidLimit(t *testing.T) {
	uc := NewJobListUsecase(mockJobRepo{}, mockJobSkillRepo{}, nil)
	_, _, err := uc.ListJobs(context.Background(), JobListParams{Limit: -1, Offset: 0})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestJobListUsecase_ListJobs_Success(t *testing.T) {
	jobID := uuid.New()
	posted := time.Now().UTC()
	uc := NewJobListUsecase(
		mockJobRepo{items: []repository.JobListRow{{
			ID:          jobID,
			Title:       "Backend Engineer",
			Company:     "Acme",
			Location:    "Jakarta",
			Description: "desc",
			PostedAt:    &posted,
		}}},
		mockJobSkillRepo{m: map[uuid.UUID][]repository.JobSkillRequirement{jobID: {
			{SkillID: uuid.New(), SkillName: "Go", ImportanceWeight: 5},
			{SkillID: uuid.New(), SkillName: "PostgreSQL", ImportanceWeight: 4},
		}}},
		nil,
	)

	items, partial, err := uc.ListJobs(context.Background(), JobListParams{Limit: 20, Offset: 0})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if partial {
		t.Fatalf("expected partial=false")
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].JobID != jobID {
		t.Fatalf("unexpected job id")
	}
	if len(items[0].Skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(items[0].Skills))
	}
}
