package dto

import "github.com/google/uuid"

type JobListResponse struct {
	JobID       uuid.UUID `json:"job_id"`
	Title       string    `json:"title"`
	CompanyName string    `json:"company_name"`
	Location    string    `json:"location"`
	Description string    `json:"description"`
	Skills      []string  `json:"skills"`
	PostedDate  string    `json:"posted_date"`
}
