package match

import (
	"time"

	"github.com/google/uuid"
)

type JobMatch struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	JobID      uuid.UUID
	MatchScore *float64
	MatchedAt  *time.Time
}
