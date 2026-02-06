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
	userBySkillID := make(map[uuid.UUID]UserSkillV2, len(userSkills))
	for _, us := range userSkills {
		if us.SkillID == uuid.Nil {
			continue
		}
		userBySkillID[us.SkillID] = us
	}

	mandatory := make([]JobRequirementV2, 0)
	optional := make([]JobRequirementV2, 0)
	for _, r := range reqs {
		if r.SkillID == uuid.Nil {
			continue
		}
		if resolveIsMandatoryV2(r) {
			mandatory = append(mandatory, r)
		} else {
			optional = append(optional, r)
		}
	}

	var mandatoryTotal float64
	var optionalTotal float64
	var expTotal float64

	matched := make([]MatchedSkillV2, 0, len(reqs))
	missing := make([]MissingSkillV2, 0)

	mandatoryPer := 0.0
	if len(mandatory) > 0 {
		mandatoryPer = 60.0 / float64(len(mandatory))
	}
	optionalPer := 0.0
	if len(optional) > 0 {
		optionalPer = 30.0 / float64(len(optional))
	}

	expDenom := 0
	expSum := 0.0

	mandatoryMissing := false

	for _, r := range mandatory {
		us, ok := userBySkillID[r.SkillID]
		if !ok {
			mandatoryMissing = true
			missing = append(missing, MissingSkillV2{SkillID: r.SkillID, SkillName: r.SkillName, IsMandatory: true})
			expDenom++
			continue
		}

		contrib := scoreRequirementV2(us, r, mandatoryPer)
		mandatoryTotal += contrib
		matched = append(matched, MatchedSkillV2{SkillID: r.SkillID, SkillName: r.SkillName, ScoreContribution: int(math.Round(contrib))})

		expDenom++
		expSum += expRatioV2(us, r)
	}

	for _, r := range optional {
		us, ok := userBySkillID[r.SkillID]
		if !ok {
			missing = append(missing, MissingSkillV2{SkillID: r.SkillID, SkillName: r.SkillName, IsMandatory: false})
			expDenom++
			continue
		}

		contrib := scoreRequirementV2(us, r, optionalPer)
		optionalTotal += contrib
		matched = append(matched, MatchedSkillV2{SkillID: r.SkillID, SkillName: r.SkillName, ScoreContribution: int(math.Round(contrib))})

		expDenom++
		expSum += expRatioV2(us, r)
	}

	if expDenom > 0 {
		expTotal = 10.0 * (expSum / float64(expDenom))
	}

	total := mandatoryTotal + optionalTotal + expTotal
	score := int(math.Round(total))
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

func scoreRequirementV2(us UserSkillV2, r JobRequirementV2, weight float64) float64 {
	reqLvl := resolveRequiredLevelV2(r)
	usrLvl := clampInt(us.ProficiencyLevel, 0, 5)
	if usrLvl <= 0 {
		return 0
	}
	if reqLvl <= 0 {
		return weight
	}
	if usrLvl >= reqLvl {
		return weight
	}
	return weight * (float64(usrLvl) / float64(reqLvl))
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
