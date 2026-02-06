package user

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Profile struct {
	ID              uuid.UUID
	UserID          *uuid.UUID
	FullName        *string
	ExperienceLevel *string
	PreferredRoles  []string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
