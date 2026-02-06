package dto

import (
	"time"

	"github.com/google/uuid"
)

type UserProfileResponse struct {
	ID              uuid.UUID `json:"id"`
	Email           string    `json:"email"`
	FullName        *string   `json:"full_name"`
	ExperienceLevel *string   `json:"experience_level"`
	PreferredRoles  []string  `json:"preferred_roles"`
	CreatedAt       time.Time `json:"created_at"`
}
