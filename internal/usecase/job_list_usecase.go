package usecase

import (
	"context"
	"strings"
	"time"

	"skill-sync/internal/repository"

	"github.com/google/uuid"
)

type JobListParams struct {
	Title       string
	CompanyName string
	Location    string
	Skills      []string
	Limit       int
	Offset      int
}

type JobListItem struct {
	JobID       uuid.UUID
	Title       string
	CompanyName string
	Location    string
	Description string
	Skills      []string
	PostedAt    *time.Time
}

type JobListUsecase interface {
	ListJobs(ctx context.Context, params JobListParams) ([]JobListItem, error)
}

type JobList struct {
	jobs      repository.JobRepository
	jobSkills repository.JobSkillRepository
}

func NewJobListUsecase(jobs repository.JobRepository, jobSkills repository.JobSkillRepository) *JobList {
	return &JobList{jobs: jobs, jobSkills: jobSkills}
}

func (u *JobList) ListJobs(ctx context.Context, params JobListParams) ([]JobListItem, error) {
	limit := params.Limit
	if limit == 0 {
		limit = 20
	}
	if limit < 0 || limit > 50 {
		return nil, ErrInvalidInput
	}
	offset := params.Offset
	if offset < 0 {
		return nil, ErrInvalidInput
	}

	skills := make([]string, 0, len(params.Skills))
	for _, s := range params.Skills {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		skills = append(skills, s)
	}

	rows, err := u.jobs.ListJobsForListing(ctx, repository.JobListFilter{
		Title:       params.Title,
		CompanyName: params.CompanyName,
		Location:    params.Location,
		Skills:      skills,
		Limit:       limit,
		Offset:      offset,
	})
	if err != nil {
		return nil, ErrInternal
	}

	jobIDs := make([]uuid.UUID, 0, len(rows))
	for _, r := range rows {
		if r.ID == uuid.Nil {
			continue
		}
		jobIDs = append(jobIDs, r.ID)
	}

	reqsByJobID, err := u.jobSkills.FindByJobIDs(ctx, jobIDs)
	if err != nil {
		return nil, ErrInternal
	}

	out := make([]JobListItem, 0, len(rows))
	for _, r := range rows {
		reqs := reqsByJobID[r.ID]
		jobSkills := make([]string, 0, len(reqs))
		for _, it := range reqs {
			if it.SkillName == "" {
				continue
			}
			jobSkills = append(jobSkills, it.SkillName)
		}

		out = append(out, JobListItem{
			JobID:       r.ID,
			Title:       r.Title,
			CompanyName: r.Company,
			Location:    r.Location,
			Description: r.Description,
			Skills:      jobSkills,
			PostedAt:    r.PostedAt,
		})
	}
	return out, nil
}
