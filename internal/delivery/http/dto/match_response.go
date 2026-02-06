package dto

import "github.com/google/uuid"

type MatchSkillResponse struct {
	SkillID           uuid.UUID `json:"skill_id"`
	SkillName         string    `json:"skill_name"`
	ScoreContribution int       `json:"score_contribution"`
}

type MissingSkillResponse struct {
	SkillID      uuid.UUID `json:"skill_id"`
	SkillName    string    `json:"skill_name"`
	IsMandatory  bool      `json:"is_mandatory"`
}

type MatchingResultResponse struct {
	MatchScore       int                   `json:"match_score"`
	MandatoryMissing bool                  `json:"mandatory_missing"`
	MatchedSkills    []MatchSkillResponse  `json:"matched_skills"`
	MissingSkills    []MissingSkillResponse `json:"missing_skills"`
}
