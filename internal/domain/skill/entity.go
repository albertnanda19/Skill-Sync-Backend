package skill

import (
	"time"

	"github.com/google/uuid"
)

type Skill struct {
	ID        uuid.UUID
	Name      string
	Category  string
	CreatedAt time.Time
}

type UserSkill struct {
	ID               uuid.UUID
	UserID           uuid.UUID
	SkillID          uuid.UUID
	ProficiencyLevel *int16
	CreatedAt        time.Time
}

type JobSkill struct {
	ID               uuid.UUID
	JobID            uuid.UUID
	SkillID          uuid.UUID
	ImportanceWeight *int16
}
