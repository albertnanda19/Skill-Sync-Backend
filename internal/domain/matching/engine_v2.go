package matching

import (
	"math"

	"github.com/google/uuid"
)

type UserSkillV2 struct {
	SkillID          uuid.UUID
	SkillName        string
	ProficiencyLevel int
	YearsExperience  int
}

type JobRequirementV2 struct {
	SkillID          uuid.UUID
	SkillName        string
	RequiredLevel    *int
	IsMandatory      *bool
	RequiredYears    *int
	ImportanceWeight int
}

type MatchedSkillV2 struct {
	SkillID           uuid.UUID
	SkillName         string
	ScoreContribution int
}

type MissingSkillV2 struct {
	SkillID     uuid.UUID
	SkillName   string
	IsMandatory bool
}

type ResultV2 struct {
	MatchScore       int
	MandatoryMissing bool
	MatchedSkills    []MatchedSkillV2
	MissingSkills    []MissingSkillV2
}

func CalculateV2(userSkills []UserSkillV2, reqs []JobRequirementV2) ResultV2 {
	if len(userSkills) == 0 {
		return ResultV2{MatchScore: 0, MandatoryMissing: false, MatchedSkills: nil, MissingSkills: nil}
	}
	if len(reqs) == 0 {
		return ResultV2{MatchScore: 0, MandatoryMissing: false, MatchedSkills: nil, MissingSkills: nil}
	}

	userBySkillID := make(map[uuid.UUID]UserSkillV2, len(userSkills))
	for _, us := range userSkills {
		if us.SkillID == uuid.Nil {
			continue
		}
		userBySkillID[us.SkillID] = us
	}

	var totalWeight float64
	var matchedWeight float64

	mandatoryMissing := false
	matched := make([]MatchedSkillV2, 0, len(reqs))
	missing := make([]MissingSkillV2, 0)

	for _, r := range reqs {
		if r.SkillID == uuid.Nil {
			continue
		}

		w := float64(r.ImportanceWeight)
		if w <= 0 {
			continue
		}
		totalWeight += w

		us, ok := userBySkillID[r.SkillID]
		if !ok {
			isMand := resolveIsMandatoryV2(r)
			if isMand {
				mandatoryMissing = true
			}
			missing = append(missing, MissingSkillV2{SkillID: r.SkillID, SkillName: r.SkillName, IsMandatory: isMand})
			continue
		}

		reqLvl := resolveRequiredLevelV2(r)
		usrLvl := clampInt(us.ProficiencyLevel, 0, 5)
		levelRatio := 0.0
		if usrLvl > 0 {
			if reqLvl <= 0 {
				levelRatio = 1
			} else {
				levelRatio = float64(usrLvl) / float64(reqLvl)
				if levelRatio > 1 {
					levelRatio = 1
				}
				if levelRatio < 0 {
					levelRatio = 0
				}
			}
		}

		reqYears := resolveRequiredYearsV2(r)
		expRatio := 1.0
		if reqYears > 0 {
			usrYears := us.YearsExperience
			if usrYears <= 0 {
				expRatio = 0
			} else {
				expRatio = float64(usrYears) / float64(reqYears)
				if expRatio > 1 {
					expRatio = 1
				}
				if expRatio < 0 {
					expRatio = 0
				}
			}
		}

		skillScore := (levelRatio * 0.7) + (expRatio * 0.3)
		if skillScore < 0 {
			skillScore = 0
		}
		if skillScore > 1 {
			skillScore = 1
		}

		contrib := skillScore * w
		matchedWeight += contrib
		matched = append(matched, MatchedSkillV2{SkillID: r.SkillID, SkillName: r.SkillName, ScoreContribution: int(math.Round(contrib))})
	}

	if totalWeight <= 0 {
		return ResultV2{MatchScore: 0, MandatoryMissing: mandatoryMissing, MatchedSkills: matched, MissingSkills: missing}
	}

	scoreFloat := (matchedWeight / totalWeight) * 100
	score := int(math.Round(scoreFloat))
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return ResultV2{
		MatchScore:       score,
		MandatoryMissing: mandatoryMissing,
		MatchedSkills:    matched,
		MissingSkills:    missing,
	}
}

func resolveRequiredLevelV2(r JobRequirementV2) int {
	if r.RequiredLevel != nil {
		return clampInt(*r.RequiredLevel, 1, 5)
	}
	return clampInt(r.ImportanceWeight, 1, 5)
}

func resolveRequiredYearsV2(r JobRequirementV2) int {
	if r.RequiredYears != nil {
		y := *r.RequiredYears
		if y < 0 {
			return 0
		}
		return y
	}
	return resolveRequiredLevelV2(r)
}

func resolveIsMandatoryV2(r JobRequirementV2) bool {
	if r.IsMandatory != nil {
		return *r.IsMandatory
	}
	return r.ImportanceWeight >= 4
}

func expRatioV2(us UserSkillV2, r JobRequirementV2) float64 {
	reqYears := resolveRequiredYearsV2(r)
	if reqYears <= 0 {
		return 1
	}
	usrYears := us.YearsExperience
	if usrYears <= 0 {
		return 0
	}
	ratio := float64(usrYears) / float64(reqYears)
	if ratio > 1 {
		return 1
	}
	if ratio < 0 {
		return 0
	}
	return ratio
}
