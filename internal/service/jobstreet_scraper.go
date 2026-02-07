package service

import (
	"context"

	"skill-sync/internal/repository"
)

type JobStreetScraper struct {
	baseURL string
}

func NewJobStreetScraper() *JobStreetScraper {
	return &JobStreetScraper{baseURL: ""}
}

func (s *JobStreetScraper) Name() string {
	return "JobStreet"
}

func (s *JobStreetScraper) Search(ctx context.Context, params SearchParams) ([]repository.JobUpsert, error) {
	_ = ctx
	_ = params
	return nil, nil
}
