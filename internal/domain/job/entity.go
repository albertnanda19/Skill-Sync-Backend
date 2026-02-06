package job

import (
	"time"

	"github.com/google/uuid"
)

type Source struct {
	ID        uuid.UUID
	Name      *string
	BaseURL   *string
	CreatedAt time.Time
}

type Job struct {
	ID             uuid.UUID
	SourceID       *uuid.UUID
	ExternalJobID  *string
	Title          *string
	Company        *string
	Location       *string
	EmploymentType *string
	Description    *string
	RawDescription *string
	PostedAt       *time.Time
	ScrapedAt      *time.Time
	CreatedAt      time.Time
}

type ScrapeRun struct {
	ID         uuid.UUID
	SourceID   *uuid.UUID
	StartedAt  *time.Time
	FinishedAt *time.Time
	Status     *string
}

type ScrapeLog struct {
	ID          uuid.UUID
	ScrapeRunID uuid.UUID
	Level       *string
	Message     *string
	CreatedAt   time.Time
}
