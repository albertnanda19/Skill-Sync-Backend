package service

import (
	"context"
	"strings"

	"skill-sync/internal/repository"
)

type DevtoScraper struct {
	apiBase  string
	siteBase string
}

func NewDevtoScraper() *DevtoScraper {
	return &DevtoScraper{
		apiBase:  "",
		siteBase: "",
	}
}

func (s *DevtoScraper) Name() string {
	return "Dev.to Jobs"
}

type devtoListing struct {
	ID           int     `json:"id"`
	Title        string  `json:"title"`
	Category     string  `json:"category"`
	Organization string  `json:"organization_name"`
	Company      string  `json:"company_name"`
	Location     string  `json:"location"`
	PublishedAt  *string `json:"published_at"`
	URL          string  `json:"url"`
}

func (s *DevtoScraper) Search(ctx context.Context, params SearchParams) ([]repository.JobUpsert, error) {
	_ = ctx
	_ = params
	return nil, nil
}

func pickNonEmpty(a, b string) string {
	a = strings.TrimSpace(a)
	if a != "" {
		return a
	}
	return strings.TrimSpace(b)
}

func filterJobsByParams(in []repository.JobUpsert, params SearchParams) []repository.JobUpsert {
	titleQ := strings.ToLower(strings.TrimSpace(params.Title))
	companyQ := strings.ToLower(strings.TrimSpace(params.CompanyName))
	locQ := strings.ToLower(strings.TrimSpace(params.Location))
	skills := make([]string, 0, len(params.Skills))
	for _, s := range params.Skills {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" {
			continue
		}
		skills = append(skills, s)
	}

	out := make([]repository.JobUpsert, 0, len(in))
	for _, j := range in {
		if titleQ != "" && !strings.Contains(strings.ToLower(j.Title), titleQ) {
			continue
		}
		if companyQ != "" && !strings.Contains(strings.ToLower(j.Company), companyQ) {
			continue
		}
		if locQ != "" && !strings.Contains(strings.ToLower(j.Location), locQ) {
			continue
		}
		if len(skills) > 0 {
			combined := strings.ToLower(j.Title + " " + j.Description + " " + j.RawDescription)
			ok := false
			for _, s := range skills {
				if strings.Contains(combined, s) {
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
		}
		out = append(out, j)
		if len(out) >= 50 {
			break
		}
	}
	return out
}
