package usecase

import (
	"context"
	"errors"

	"skill-sync/internal/domain/matching"
	"skill-sync/internal/repository"

	"github.com/google/uuid"
)

var (
	ErrJobNotFound             = errors.New("Job not found")
	ErrUserSkillProfileEmpty   = errors.New("User skill profile empty")
)

type MatchingUsecase interface {
	CalculateMatch(ctx context.Context, userID, jobID uuid.UUID) (matching.Result, error)
}

type Matching struct {
	jobs      repository.JobRepository
	jobSkills repository.JobSkillRepository
	userSkills repository.UserSkillRepository
}

func NewMatchingUsecase(jobs repository.JobRepository, jobSkills repository.JobSkillRepository, userSkills repository.UserSkillRepository) *Matching {
	return &Matching{jobs: jobs, jobSkills: jobSkills, userSkills: userSkills}
}

func (u *Matching) CalculateMatch(ctx context.Context, userID, jobID uuid.UUID) (matching.Result, error) {
	if userID == uuid.Nil {
		return matching.Result{}, ErrUnauthorized
	}
	if jobID == uuid.Nil {
		return matching.Result{}, ErrJobNotFound
	}

	exists, err := u.jobs.ExistsByID(ctx, jobID)
	if err != nil {
		return matching.Result{}, ErrInternal
	}
	if !exists {
		return matching.Result{}, ErrJobNotFound
	}

	us, err := u.userSkills.FindByUserID(ctx, userID)
	if err != nil {
		return matching.Result{}, ErrInternal
	}
	if len(us) == 0 {
		return matching.Result{}, ErrUserSkillProfileEmpty
	}

	reqs, err := u.jobSkills.FindByJobID(ctx, jobID)
	if err != nil {
		return matching.Result{}, ErrInternal
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
	return res, nil
}
