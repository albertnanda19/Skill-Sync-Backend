package dto

import "github.com/google/uuid"

type JobRecommendationResponse struct {
	JobID            uuid.UUID                          `json:"job_id"`
	Title            string                             `json:"title"`
	CompanyName      string                             `json:"company_name"`
	Location         string                             `json:"location"`
	MatchScore       int                                `json:"match_score"`
	MandatoryMissing bool                               `json:"mandatory_missing"`
	MissingSkills    []JobRecommendationMissingSkillItem `json:"missing_skills"`
}

type JobRecommendationMissingSkillItem struct {
	SkillID     uuid.UUID `json:"skill_id"`
	SkillName   string    `json:"skill_name"`
	IsMandatory bool      `json:"is_mandatory"`
}
