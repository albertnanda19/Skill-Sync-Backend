package dto

import (
	"time"

	"github.com/google/uuid"
)

type JobRecommendationEnvelope struct {
	GeneratedAt          time.Time                   `json:"generated_at"`
	RecommendationSource string                      `json:"recommendation_source"`
	Jobs                 []JobRecommendationResponse `json:"jobs"`
	Message              string                      `json:"message,omitempty"`
}

type JobRecommendationResponse struct {
	JobID            uuid.UUID                           `json:"job_id"`
	Title            string                              `json:"title"`
	CompanyName      string                              `json:"company_name"`
	Location         string                              `json:"location"`
	JobURL           string                              `json:"job_url"`
	Source           string                              `json:"source"`
	MatchScore       int                                 `json:"match_score"`
	MatchReason      []string                            `json:"match_reason"`
	MandatoryMissing bool                                `json:"mandatory_missing"`
	MissingSkills    []JobRecommendationMissingSkillItem `json:"missing_skills"`
}

type JobRecommendationMissingSkillItem struct {
	SkillID     uuid.UUID `json:"skill_id"`
	SkillName   string    `json:"skill_name"`
	IsMandatory bool      `json:"is_mandatory"`
}
