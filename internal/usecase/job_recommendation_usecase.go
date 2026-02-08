package usecase

import (
	"context"
	"errors"
	"log"
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
	jobs        repository.JobRepository
	jobSkills   repository.JobSkillRepository
	jobSkillsV2 repository.JobSkillV2Repository
	userSkills  repository.UserSkillRepository
}

func NewJobRecommendationUsecase(jobs repository.JobRepository, jobSkills repository.JobSkillRepository, jobSkillsV2 repository.JobSkillV2Repository, userSkills repository.UserSkillRepository) *JobRecommendation {
	return &JobRecommendation{jobs: jobs, jobSkills: jobSkills, jobSkillsV2: jobSkillsV2, userSkills: userSkills}
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
	log.Printf("user skills count: %d", len(us))

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

	engineUserSkills := make([]matching.UserSkillV2, 0, len(us))
	for _, it := range us {
		engineUserSkills = append(engineUserSkills, matching.UserSkillV2{
			SkillID:          it.SkillID,
			SkillName:        it.SkillName,
			ProficiencyLevel: it.ProficiencyLevel,
			YearsExperience:  it.YearsExperience,
		})
	}

	out := make([]JobRecommendationItem, 0, len(jobs))
	for _, j := range jobs {
		var reqsV2 []repository.JobSkillRequirementV2
		if u.jobSkillsV2 != nil {
			reqsV2, err = u.jobSkillsV2.FindByJobIDV2(ctx, j.ID)
			if err != nil {
				return nil, ErrInternal
			}
		} else {
			// Backward compatibility if not wired: map v1 requirements into v2 inputs.
			fallback, ferr := u.jobSkills.FindByJobID(ctx, j.ID)
			if ferr != nil {
				return nil, ErrInternal
			}
			reqsV2 = make([]repository.JobSkillRequirementV2, 0, len(fallback))
			for _, r := range fallback {
				reqsV2 = append(reqsV2, repository.JobSkillRequirementV2{
					SkillID:          r.SkillID,
					SkillName:        r.SkillName,
					RequiredLevel:    nil,
					IsMandatory:      nil,
					RequiredYears:    nil,
					ImportanceWeight: r.ImportanceWeight,
				})
			}
		}

		log.Printf("job %s required skills: %d", j.ID, len(reqsV2))
		if len(reqsV2) == 0 {
			continue
		}

		totalWeight := 0
		engineReqs := make([]matching.JobRequirementV2, 0, len(reqsV2))
		for _, r := range reqsV2 {
			if r.ImportanceWeight > 0 {
				totalWeight += r.ImportanceWeight
			}
			engineReqs = append(engineReqs, matching.JobRequirementV2{
				SkillID:          r.SkillID,
				SkillName:        r.SkillName,
				RequiredLevel:    r.RequiredLevel,
				IsMandatory:      r.IsMandatory,
				RequiredYears:    r.RequiredYears,
				ImportanceWeight: r.ImportanceWeight,
			})
		}
		if totalWeight <= 0 {
			continue
		}

		res := matching.CalculateV2(engineUserSkills, engineReqs)
		log.Printf("job %s score %d", j.ID, res.MatchScore)
		if res.MatchScore < minScore {
			continue
		}

		missing := make([]matching.MissingSkill, 0, len(res.MissingSkills))
		for _, ms := range res.MissingSkills {
			missing = append(missing, matching.MissingSkill{SkillID: ms.SkillID, SkillName: ms.SkillName, IsMandatory: ms.IsMandatory})
		}

		out = append(out, JobRecommendationItem{
			JobID:            j.ID,
			Title:            j.Title,
			CompanyName:      j.Company,
			Location:         j.Location,
			MatchScore:       res.MatchScore,
			MandatoryMissing: res.MandatoryMissing,
			MissingSkills:    missing,
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
