package dto

import "github.com/google/uuid"

type UserSkillResponse struct {
	ID               uuid.UUID `json:"id"`
	SkillID          uuid.UUID `json:"skill_id"`
	SkillName        string    `json:"skill_name"`
	ProficiencyLevel int       `json:"proficiency_level"`
	YearsExperience  int       `json:"years_experience"`
}
