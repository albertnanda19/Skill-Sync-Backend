package matching

import (
	"math"

	"github.com/google/uuid"
)

type UserSkill struct {
	SkillID          uuid.UUID
	SkillName        string
	ProficiencyLevel int
	YearsExperience  int
}

type JobRequirement struct {
	SkillID       uuid.UUID
	SkillName     string
	RequiredLevel int
	IsMandatory   bool
	RequiredYears int
}

type MatchedSkill struct {
	SkillID           uuid.UUID
	SkillName         string
	ScoreContribution int
}

type MissingSkill struct {
	SkillID     uuid.UUID
	SkillName   string
	IsMandatory bool
}

type Result struct {
	MatchScore       int
	MandatoryMissing bool
	MatchedSkills    []MatchedSkill
	MissingSkills    []MissingSkill
}

func Calculate(userSkills []UserSkill, reqs []JobRequirement) Result {
	userBySkillID := make(map[uuid.UUID]UserSkill, len(userSkills))
	for _, us := range userSkills {
		if us.SkillID == uuid.Nil {
			continue
		}
		userBySkillID[us.SkillID] = us
	}

	mandatory := make([]JobRequirement, 0)
	optional := make([]JobRequirement, 0)
	for _, r := range reqs {
		if r.SkillID == uuid.Nil {
			continue
		}
		if r.IsMandatory {
			mandatory = append(mandatory, r)
		} else {
			optional = append(optional, r)
		}
	}

	var mandatoryTotal float64
	var optionalTotal float64
	var expTotal float64

	matched := make([]MatchedSkill, 0, len(reqs))
	missing := make([]MissingSkill, 0)

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

	scoreReq := func(us UserSkill, r JobRequirement, weight float64) float64 {
		reqLvl := clampInt(r.RequiredLevel, 1, 5)
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

	for _, r := range mandatory {
		us, ok := userBySkillID[r.SkillID]
		if !ok {
			mandatoryMissing = true
			missing = append(missing, MissingSkill{SkillID: r.SkillID, SkillName: r.SkillName, IsMandatory: true})
			expDenom++
			continue
		}

		contrib := scoreReq(us, r, mandatoryPer)
		mandatoryTotal += contrib
		matched = append(matched, MatchedSkill{SkillID: r.SkillID, SkillName: r.SkillName, ScoreContribution: int(math.Round(contrib))})

		expDenom++
		expSum += expRatio(us, r)
	}

	for _, r := range optional {
		us, ok := userBySkillID[r.SkillID]
		if !ok {
			missing = append(missing, MissingSkill{SkillID: r.SkillID, SkillName: r.SkillName, IsMandatory: false})
			expDenom++
			continue
		}

		contrib := scoreReq(us, r, optionalPer)
		optionalTotal += contrib
		matched = append(matched, MatchedSkill{SkillID: r.SkillID, SkillName: r.SkillName, ScoreContribution: int(math.Round(contrib))})

		expDenom++
		expSum += expRatio(us, r)
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

	return Result{
		MatchScore:       score,
		MandatoryMissing: mandatoryMissing,
		MatchedSkills:    matched,
		MissingSkills:    missing,
	}
}

func expRatio(us UserSkill, r JobRequirement) float64 {
	reqYears := r.RequiredYears
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

func clampInt(v, minV, maxV int) int {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}
