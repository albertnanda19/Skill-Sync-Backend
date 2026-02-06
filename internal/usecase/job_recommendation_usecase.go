package usecase

import (
	"context"
	"errors"
	"sort"

	"skill-sync/internal/domain/matching"
	"skill-sync/internal/repository"

	"github.com/google/uuid"
)

var (
	ErrNoJobsFound = errors.New("No jobs found")
)

type JobRecommendationParams struct {
	Limit    int
	Offset   int
	MinScore int
}

type JobRecommendationUsecase interface {
	GetRecommendations(ctx context.Context, userID uuid.UUID, params JobRecommendationParams) ([]JobRecommendationItem, error)
}

type JobRecommendationItem struct {
	JobID            uuid.UUID
	Title            string
	CompanyName      string
	Location         string
	MatchScore       int
	MandatoryMissing bool
	MissingSkills    []matching.MissingSkill
}

type JobRecommendation struct {
	jobs       repository.JobRepository
	jobSkills  repository.JobSkillRepository
	userSkills repository.UserSkillRepository
}

func NewJobRecommendationUsecase(jobs repository.JobRepository, jobSkills repository.JobSkillRepository, userSkills repository.UserSkillRepository) *JobRecommendation {
	return &JobRecommendation{jobs: jobs, jobSkills: jobSkills, userSkills: userSkills}
}

func (u *JobRecommendation) GetRecommendations(ctx context.Context, userID uuid.UUID, params JobRecommendationParams) ([]JobRecommendationItem, error) {
	if userID == uuid.Nil {
		return nil, ErrUnauthorized
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	offset := params.Offset
	if offset < 0 {
		offset = 0
	}
	minScore := params.MinScore
	if minScore < 0 {
		minScore = 0
	}

	us, err := u.userSkills.FindByUserID(ctx, userID)
	if err != nil {
		return nil, ErrInternal
	}
	if len(us) == 0 {
		return nil, ErrUserSkillProfileEmpty
	}

	jobs, err := u.jobs.ListJobs(ctx, limit, offset)
	if err != nil {
		return nil, ErrInternal
	}
	if len(jobs) == 0 {
		return nil, ErrNoJobsFound
	}

	jobIDs := make([]uuid.UUID, 0, len(jobs))
	for _, j := range jobs {
		if j.ID == uuid.Nil {
			continue
		}
		jobIDs = append(jobIDs, j.ID)
	}

	reqsByJobID, err := u.jobSkills.FindByJobIDs(ctx, jobIDs)
	if err != nil {
		return nil, ErrInternal
	}

	engineUserSkills := make([]matching.UserSkill, 0, len(us))
	for _, it := range us {
		engineUserSkills = append(engineUserSkills, matching.UserSkill{
			SkillID:          it.SkillID,
			SkillName:        it.SkillName,
			ProficiencyLevel: it.ProficiencyLevel,
			YearsExperience:  it.YearsExperience,
		})
	}

	out := make([]JobRecommendationItem, 0, len(jobs))
	for _, j := range jobs {
		reqs := reqsByJobID[j.ID]
		engineReqs := make([]matching.JobRequirement, 0, len(reqs))
		for _, r := range reqs {
			requiredLevel := r.ImportanceWeight
			if requiredLevel < 1 {
				requiredLevel = 1
			}
			if requiredLevel > 5 {
				requiredLevel = 5
			}
			engineReqs = append(engineReqs, matching.JobRequirement{
				SkillID:       r.SkillID,
				SkillName:     r.SkillName,
				RequiredLevel: requiredLevel,
				IsMandatory:   requiredLevel >= 4,
				RequiredYears: requiredLevel,
			})
		}

		res := matching.Calculate(engineUserSkills, engineReqs)
		if res.MatchScore < minScore {
			continue
		}

		out = append(out, JobRecommendationItem{
			JobID:            j.ID,
			Title:            j.Title,
			CompanyName:      j.Company,
			Location:         j.Location,
			MatchScore:       res.MatchScore,
			MandatoryMissing: res.MandatoryMissing,
			MissingSkills:    res.MissingSkills,
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		return out[i].MatchScore > out[j].MatchScore
	})

	if len(out) == 0 {
		return nil, ErrNoJobsFound
	}

	return out, nil
}
