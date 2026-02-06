package usecase

import (
	"context"

	"skill-sync/internal/domain/matching"
	"skill-sync/internal/repository"

	"github.com/google/uuid"
)

type MatchingUsecaseV2 interface {
	CalculateMatchV2(ctx context.Context, userID, jobID uuid.UUID) (matching.ResultV2, error)
}

type MatchingV2 struct {
	jobs       repository.JobRepository
	jobSkillsV2 repository.JobSkillV2Repository
	userSkills repository.UserSkillRepository
}

func NewMatchingUsecaseV2(jobs repository.JobRepository, jobSkillsV2 repository.JobSkillV2Repository, userSkills repository.UserSkillRepository) *MatchingV2 {
	return &MatchingV2{jobs: jobs, jobSkillsV2: jobSkillsV2, userSkills: userSkills}
}

func (u *MatchingV2) CalculateMatchV2(ctx context.Context, userID, jobID uuid.UUID) (matching.ResultV2, error) {
	if userID == uuid.Nil {
		return matching.ResultV2{}, ErrUnauthorized
	}
	if jobID == uuid.Nil {
		return matching.ResultV2{}, ErrJobNotFound
	}

	exists, err := u.jobs.ExistsByID(ctx, jobID)
	if err != nil {
		return matching.ResultV2{}, ErrInternal
	}
	if !exists {
		return matching.ResultV2{}, ErrJobNotFound
	}

	us, err := u.userSkills.FindByUserID(ctx, userID)
	if err != nil {
		return matching.ResultV2{}, ErrInternal
	}
	if len(us) == 0 {
		return matching.ResultV2{}, ErrUserSkillProfileEmpty
	}

	reqs, err := u.jobSkillsV2.FindByJobIDV2(ctx, jobID)
	if err != nil {
		return matching.ResultV2{}, ErrInternal
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

	engineReqs := make([]matching.JobRequirementV2, 0, len(reqs))
	for _, r := range reqs {
		engineReqs = append(engineReqs, matching.JobRequirementV2{
			SkillID:          r.SkillID,
			SkillName:        r.SkillName,
			RequiredLevel:    r.RequiredLevel,
			IsMandatory:      r.IsMandatory,
			RequiredYears:    r.RequiredYears,
			ImportanceWeight: r.ImportanceWeight,
		})
	}

	res := matching.CalculateV2(engineUserSkills, engineReqs)
	return res, nil
}
