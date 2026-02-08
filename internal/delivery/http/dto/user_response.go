package dto

import (
	"time"

	"github.com/google/uuid"
)

type UserProfileResponse struct {
	ID                 uuid.UUID `json:"id"`
	Email              string    `json:"email"`
	FullName           *string   `json:"full_name"`
	ExperienceLevel    *string   `json:"experience_level"`
	PreferenceLocation *string   `json:"preference_location"`
	PreferredRoles     []string  `json:"preferred_roles"`
	CreatedAt          time.Time `json:"created_at"`
}

type UserMeSkillItem struct {
	SkillName        string `json:"skill_name"`
	ProficiencyLevel int    `json:"proficiency_level"`
	YearsExperience  int    `json:"years_experience"`
}

type UserMeSkillsSection struct {
	Core       []UserMeSkillItem `json:"core"`
	Developing []UserMeSkillItem `json:"developing"`
}

type UserMePreferencesSection struct {
	PreferredRoles     []string `json:"preferred_roles"`
	PreferenceLocation *string  `json:"preference_location"`
	ExperienceLevel    *string  `json:"experience_level"`
}

type UserMeResponse struct {
	ID          uuid.UUID                `json:"id"`
	Email       string                   `json:"email"`
	FullName    *string                  `json:"full_name"`
	CreatedAt   time.Time                `json:"created_at"`
	Skills      UserMeSkillsSection      `json:"skills"`
	Preferences UserMePreferencesSection `json:"preferences"`
}
