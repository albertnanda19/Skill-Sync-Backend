package service

import (
	"context"

	"skill-sync/internal/repository"
)

type GlintsScraper struct {
	siteBase string
}

func NewGlintsScraper() *GlintsScraper {
	return &GlintsScraper{
		siteBase: "",
	}
}

func (s *GlintsScraper) Name() string {
	return "Glints"
}

type glintsJobItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Location string `json:"location"`
	Company  string `json:"company"`
	URL      string `json:"url"`
	Slug     string `json:"slug"`
}

func (s *GlintsScraper) Search(ctx context.Context, params SearchParams) ([]repository.JobUpsert, error) {
	_ = ctx
	_ = params
	return nil, nil
}
