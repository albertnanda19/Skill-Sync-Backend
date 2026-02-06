package dto

import "github.com/google/uuid"

type MatchSkillResponseV2 struct {
	SkillID           uuid.UUID `json:"skill_id"`
	SkillName         string    `json:"skill_name"`
	ScoreContribution int       `json:"score_contribution"`
}

type MissingSkillResponseV2 struct {
	SkillID     uuid.UUID `json:"skill_id"`
	SkillName   string    `json:"skill_name"`
	IsMandatory bool      `json:"is_mandatory"`
}

type MatchingResultResponseV2 struct {
	MatchScore       int                     `json:"match_score"`
	MandatoryMissing bool                    `json:"mandatory_missing"`
	MatchedSkills    []MatchSkillResponseV2  `json:"matched_skills"`
	MissingSkills    []MissingSkillResponseV2 `json:"missing_skills"`
}
