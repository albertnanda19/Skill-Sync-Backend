package dto

import "github.com/google/uuid"

type JobSourceResponse struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	BaseURL string    `json:"base_url"`
}
